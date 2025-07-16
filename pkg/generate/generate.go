// Package generate holds the primary code for wordlist generation
package generate

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"ponder/pkg/models"
	"ponder/pkg/utils"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// CreateWizardWordlist processes the source file in chunks, removes trailing digits from strings,
// and writes the processed content to the target file in a memory-efficient manner.
//
// Args:
// sourcePATH (string): The path to the source file.
// targetPATH (string): The path to the target file.
//
// Returns:
// error: An error if one occurred.
func CreateWizardWordlist(sourcePATH string, targetPATH string) error {
	sourceFile, err := os.Open(sourcePATH)
	if err != nil {
		utils.LogInternalEvent("Error opening file in wordlist generation", err.Error())
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.OpenFile(targetPATH, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		utils.LogInternalEvent("Error opening file in wordlist generation", err.Error())
		return err
	}
	defer targetFile.Close()

	// This buffer size is the maximum size that can be processed in a single
	// chunk. This is to prevent memory exhaustion when processing large files.
	//
	// The buffer size is set to 256KB, which is a reasonable size for most
	// systems. This value can be adjusted based on the system's memory
	// capacity. Values of 1MB to 8MB are reasonable for systems.
	//
	// The highest tested value was 1MB. The size was reduced to 256KB to
	// ensure completion at the expense of speed because other operations could
	// use the memory for more valuable tasks like deduplication/sorting.
	buffer := make([]byte, 256*1024)

	utils.LogInternalEvent("Processing source file", fmt.Sprintf("Chunk size: %d bytes.", len(buffer)))
	for {
		n, err := sourceFile.Read(buffer)
		if err != nil && err != io.EOF {
			utils.LogInternalEvent("Error reading file in wordlist generation", err.Error())
			return err
		}
		if n == 0 {
			break
		}

		processedChunk := buffer[:n]
		processedChunk = models.GenerateNGramSliceBytes(processedChunk, 1, 5)
		preparedLines := PrepareStringForTransformations(processedChunk)
		processedChunk = []byte(strings.Join(preparedLines, "\n"))
		processedChunk = filterLines(processedChunk)
		processedChunk = removeTrailingDigits(processedChunk)
		processedChunk = removeLeadingDigits(processedChunk)
		processedChunk = models.EnforceLengthRange(processedChunk, 4, 32)
		nGramChunk := filterLines(processedChunk)

		if _, err := targetFile.Write(nGramChunk); err != nil {
			utils.LogInternalEvent("Error writing to file in wordlist generation", err.Error())
			return err
		}
	}

	utils.LogInternalEvent("Sorting wordlist by frequency", fmt.Sprintf("Target: %s.", targetPATH))

	// Some deduplication from the function below 
	if err := utils.SortByAproxFrequency(targetPATH); err != nil {
		utils.LogInternalEvent("Error sorting wordlist by frequency in wordlist generation", err.Error())
		return err
	}

	return nil
}

// PrepareStringForTransformations processes each line in the input byte slice,
// removes unwanted characters, normalizes each line, and generates various
// transformed versions for each line.
//
// Args:
//
//	data ([]byte): The byte slice containing lines to process.
//
// Returns:
//
//	[]string: A flattened slice of all prepared string variants for all lines.
func PrepareStringForTransformations(data []byte) []string {
	// Convert the byte slice to a string for line-wise processing
	input := string(data)
	scanner := bufio.NewScanner(strings.NewReader(input))

	var results []string

	for scanner.Scan() {
		line := scanner.Text()

		// Remove unwanted characters
		clean := strings.ReplaceAll(line, "\x00", "")
		clean = strings.ReplaceAll(clean, "\n", "")
		clean = strings.ReplaceAll(clean, "\t", "")
		clean = strings.ReplaceAll(clean, "\r", "")
		clean = strings.ReplaceAll(clean, "\f", "")
		clean = strings.ReplaceAll(clean, "\v", "")
		clean = RemoveControlChars(clean)
		clean = strings.ToLower(clean)

		if strings.Contains(clean, " ") {
			results = append(results, strings.ReplaceAll(
				cases.Title(language.Und, cases.NoLower).String(clean), " ", ""))
		} else {
			results = append(results, strings.ReplaceAll(clean, " ", ""))
		}
	}

	return results
}

// RemoveControlChars removes all non-printable ASCII characters from a string,
// leaving only characters in the printable range (ASCII 32-126).
func RemoveControlChars(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 32 && r <= 126 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// filterLines checks each line and skips those that consist only of digits or special characters.
//
// Args:
// data ([]byte): The byte slice containing the data to be processed.
//
// Returns:
// []byte: The processed byte slice with filtered lines.
func filterLines(data []byte) []byte {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var result strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if utils.IsAllDigitsOrSpecialChars(line) {
			continue
		}
		if utils.ContainsOnlyASCII(line) == false {
			continue
		}
		if utils.LikelyContainsWords(line) == false {
			continue
		}
		result.WriteString(line + "\n")
	}

	return []byte(result.String())
}

// removeTrailingDigits removes trailing digits from each line in the given byte slice.
//
// Args:
// data ([]byte): The byte slice containing the data to be processed.
//
// Returns:
// []byte: The processed byte slice with trailing digits removed.
func removeTrailingDigits(data []byte) []byte {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var result strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		processedLine := strings.TrimRightFunc(line, func(r rune) bool {
			return r >= '0' && r <= '9'
		})
		result.WriteString(processedLine + "\n")
	}

	return []byte(result.String())
}

// removeLeadingDigits removes leading digits from each line in the given byte
// slice.
//
// Args:
// data ([]byte): The byte slice containing the data to be processed.
//
// Returns:
// []byte: The processed byte slice with leading digits removed.
func removeLeadingDigits(data []byte) []byte {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var result strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		processedLine := strings.TrimLeftFunc(line, func(r rune) bool {
			return r >= '0' && r <= '9'
		})
		result.WriteString(processedLine + "\n")
	}

	return []byte(result.String())
}
