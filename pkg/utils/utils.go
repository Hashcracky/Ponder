// Package utils contains utility functions that are used throughout the
// project
package utils

import (
	"bufio"
	"fmt"
	"os"
	"ponder/pkg/models"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"
)

// MakeFileIfNotExist creates a file if it does not exist
//
// Args:
// path (string): The path to the file
//
// Returns:
// None
func MakeFileIfNotExist(path string) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		file, err := os.Create(path)
		if err != nil {
			LogInternalEvent("Error creating file in MakeFileIfNotExist", err.Error())
			return
		}
		defer file.Close()
	}
}

// GetFirstNLines reads the first n lines from a file specified by the path
// and includes only lines that contain the specified substring (case-insensitive).
//
// Args:
// path (string): The path to the file
// n (int): The number of lines to read
// substring (string): The optional case-insensitive substring to search for
//
// Returns:
// []string: A slice of strings containing the first n lines of the file that match the substring
// error: An error if one occurred
func GetFirstNLines(path string, n int, substring ...string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := make([]string, 0, n)
	var substrLower string

	if len(substring) > 0 {
		substrLower = strings.ToLower(substring[0])
	}

	for scanner.Scan() {
		line := scanner.Text()
		if substrLower == "" || strings.Contains(strings.ToLower(line), substrLower) {
			lines = append(lines, line)
		}
		if len(lines) >= n {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

// WriteLogEntry writes a log entry to the log file and enforces a maximum log size of 5MB
//
// Args:
// entry (models.LogEntry): The log entry to write
//
// Returns:
// error: An error if one occurred
func WriteLogEntry(entry models.LogEntry) error {
	// Maximum log file size: 5MB
	const maxSize int64 = 5 * 1024 * 1024

	file, err := os.OpenFile(models.LogFile, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Check the file size
	stat, err := file.Stat()
	if err != nil {
		return err
	}

	// If the file size exceeds the maximum size, remove the oldest entry
	if stat.Size() > maxSize {
		if err := truncateLogFile(file); err != nil {
			return err
		}
	}

	writer := bufio.NewWriter(file)
	logLine := fmt.Sprintf("%s - %s: %s\n", entry.Time, entry.Event, entry.Message)
	if _, err := writer.WriteString(logLine); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	return nil
}

// truncateLogFile removes the oldest log entry from the log file
//
// Args:
// file (*os.File): The log file
//
// Returns:
// error: An error if one occurred
func truncateLogFile(file *os.File) error {
	// Read all log entries
	entries, err := ReadLogEntries()
	if err != nil {
		return err
	}

	// Remove the oldest entry
	if len(entries) > 0 {
		entries = entries[1:]
	}

	// Truncate the file and write the remaining entries
	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}

	writer := bufio.NewWriter(file)
	for _, entry := range entries {
		logLine := fmt.Sprintf("%s - %s: %s\n", entry.Time, entry.Event, entry.Message)
		if _, err := writer.WriteString(logLine); err != nil {
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	return nil
}

// ReadLogEntries reads all log entries from the log file
//
// Returns:
// ([]models.LogEntry, error): A slice of log entries and an error if one occurred
func ReadLogEntries() ([]models.LogEntry, error) {
	file, err := os.Open(models.LogFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []models.LogEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) < 2 {
			continue
		}
		timeAndEvent := strings.SplitN(parts[1], ": ", 2)
		if len(timeAndEvent) < 2 {
			continue
		}
		entries = append(entries, models.LogEntry{
			Time:    parts[0],
			Event:   timeAndEvent[0],
			Message: timeAndEvent[1],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

// LogInternalEvent logs an internal event to the log file
//
// Args:
// event (string): The event type
// message (string): The event message
//
// Returns:
// error: An error if one occurred
func LogInternalEvent(event, message string) error {
	entry := models.LogEntry{
		Time:    time.Now().Format(time.RFC3339),
		Event:   event,
		Message: message,
	}
	return WriteLogEntry(entry)
}

// IsAllDigitsOrSpecialChars checks if a string contains only digits or special characters.
//
// Args:
// s (string): The string to check
//
// Returns:
// bool: True if the string contains only digits or special characters, false otherwise
func IsAllDigitsOrSpecialChars(s string) bool {
	hasLetter := false
	for _, char := range s {
		if unicode.IsLetter(char) {
			hasLetter = true
			break
		}
	}
	return !hasLetter
}

// ContainsOnlyASCII checks if a string contains only ASCII characters.
//
// Args:
// s (string): The string to check
//
// Returns:
// bool: True if the string contains only ASCII characters, false otherwise
func ContainsOnlyASCII(s string) bool {
	for _, char := range s {
		if char > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// SortByAproxFrequency sorts the content of the target file by the frequency
// of occurrence using external sorting.
//
// The function reads the file in chunks, sorts the chunks, and writes them to
// temporary files. The sorted chunks are then merged into the target file.
// This approach is more memory-efficient than reading the entire file into
// memory but may result in duplicates due to the chunking.
//
// Args:
// targetPATH (string): The path to the file
//
// Returns:
// error: An error if one occurred
func SortByAproxFrequency(targetPATH string) error {
	tempDir := "/data/temp_chunks"
	err := os.MkdirAll(tempDir, 0755)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	LogInternalEvent("Processing file chunks", fmt.Sprintf("Processing file chunks for %s", targetPATH))
	err = processFileChunksToTempFiles(targetPATH, tempDir)
	if err != nil {
		return err
	}

	LogInternalEvent("Merging sorted chunks", fmt.Sprintf("Merging sorted chunks for %s", targetPATH))
	runtime.GC()
	err = mergeSortedChunks(tempDir, targetPATH)
	if err != nil {
		return err
	}

	return nil
}

// processFileChunksToTempFiles processes the file in smaller chunks and writes
// them to temporary files. Used in SortByAproxFrequency.
//
// Args:
// path (string): The path to the file
// tempDir (string): The temporary directory for storing files
//
// Returns:
// error: An error if one occurred
func processFileChunksToTempFiles(path, tempDir string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// This is the number of lines to read at a time. The higher the number,
	// the larger files are written to the disk. The lower the number, the more
	// files are written to the disk. Adjust as needed. Because the file is
	// read through a scanner, the memory usage is minimal. So this value is
	// just to control the size of the files written to the disk and ensure
	// inodes are not exhausted.
	//
	// Recommended: 25,000,000
	//
	chunkLineCount := 25000000
	scanner := bufio.NewScanner(file)
	chunkCounter := 0
	lines := make([]string, 0, chunkLineCount)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) >= chunkLineCount {
			err := sortAndWriteChunk(lines, tempDir, chunkCounter)
			if err != nil {
				return err
			}
			chunkCounter++
			lines = lines[:0]
			runtime.GC()
		}
	}

	if len(lines) > 0 {
		err := sortAndWriteChunk(lines, tempDir, chunkCounter)
		if err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

// mergeSortedChunks merges sorted chunks into the target file using a more
// memory-efficient approach. Used in SortByAproxFFrequency.
//
// Args:
// tempDir (string): The temporary directory containing the sorted chunks
// targetPATH (string): The path to the target file
//
// Returns:
// error: An error if one occurred
func mergeSortedChunks(tempDir, targetPATH string) error {
	files, err := os.ReadDir(tempDir)
	if err != nil {
		return err
	}

	chunkFiles := make([]*os.File, len(files))
	scanners := make([]*bufio.Scanner, len(files))
	itemsInTempDir := 0

	for i, file := range files {
		chunkFilePath := fmt.Sprintf("%s/%s", tempDir, file.Name())
		chunkFile, err := os.Open(chunkFilePath)
		if err != nil {
			LogInternalEvent("Error opening chunk file", err.Error())
			return err
		}
		chunkFiles[i] = chunkFile
		scanners[i] = bufio.NewScanner(chunkFile)
		itemsInTempDir++
	}

	outputFile, err := os.Create(targetPATH)
	if err != nil {
		LogInternalEvent("Error creating output file", err.Error())
		return err
	}
	defer outputFile.Close()

	writer := bufio.NewWriter(outputFile)
	entries := make(map[string]int)
	numberOfWrittenEntries := 0

	for i, scanner := range scanners {
		for scanner.Scan() {
			line := scanner.Text()
			entries[line]++
			// Flush the entries to the file when the map reaches a certain
			// size to avoid running out of memory. Because we are clearing the
			// map in memory, the output will contain duplicates. The higher
			// the threshold, the less duplicates, however, base memory usage
			// will also rise.
			//
			// Adjust the threshold as needed
			// Highest Approved: 250,000,000
			// 8GB Recommended: 50,000,000
			//
			if len(entries) > 50000000 {
				LogInternalEvent("Flushing entries to file", fmt.Sprintf("Flushes: %d", numberOfWrittenEntries))
				if err := flushEntriesToFile(entries, writer); err != nil {
					LogInternalEvent("Error flushing entries to file", err.Error())
					return err
				}
				entries = make(map[string]int)
				numberOfWrittenEntries++
			}
		}
		if err := scanner.Err(); err != nil {
			LogInternalEvent("Error during scanning", err.Error())
			return err
		}
		// Close file after processing
		err = chunkFiles[i].Close()
		if err != nil {
			LogInternalEvent("Error closing chunk file", err.Error())
			return err
		}
	}

	if len(entries) > 0 {
		LogInternalEvent("Flushing remaining entries to file", fmt.Sprintf("Flushes: %d", numberOfWrittenEntries))
		if err := flushEntriesToFile(entries, writer); err != nil {
			LogInternalEvent("Error flushing remaining entries to file", err.Error())
			return err
		}
	}

	if err := writer.Flush(); err != nil {
		LogInternalEvent("Error flushing writer", err.Error())
		return err
	}

	LogInternalEvent("Merge complete", fmt.Sprintf("Processed %d chunks", itemsInTempDir))
	return nil
}

// flushEntriesToFile writes the entries to the file and clears the map to free
// memory. Used in mergeSortedChunks which is used in SortByAproxFrequency.
//
// Args:
// entries (map[string]int): The entries to write
// writer (*bufio.Writer): The writer to write to the file
//
// Returns:
// error: An error if one occurred
func flushEntriesToFile(entries map[string]int, writer *bufio.Writer) error {
	type freqPair struct {
		str   string
		count int
	}

	freqPairs := make([]freqPair, 0, len(entries))
	for str, count := range entries {
		freqPairs = append(freqPairs, freqPair{str, count})
	}

	sort.Slice(freqPairs, func(i, j int) bool {
		return freqPairs[i].count > freqPairs[j].count
	})

	for _, pair := range freqPairs {
		line := strings.TrimSpace(strings.TrimSuffix(pair.str, fmt.Sprintf(" %d", pair.count)))
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}

// sortAndWriteChunk sorts a chunk of lines and writes it to a temporary file.
// Used in SortByAproxFrequency.
//
// Args:
// lines ([]string): The lines to sort and write
// tempDir (string): The temporary directory for storing files
//
// Returns:
// error: An error if one occurred
func sortAndWriteChunk(lines []string, tempDir string, chunkCounter int) error {
	sort.Strings(lines)
	tempFilePath := fmt.Sprintf("%s/chunk_%d.txt", tempDir, chunkCounter)
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return err
	}
	defer tempFile.Close()

	writer := bufio.NewWriter(tempFile)
	for _, line := range lines {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return err
		}
	}

	err = writer.Flush()
	if err != nil {
		return err
	}

	return nil
}

// LikelyContainsWords checks a string to see if there are atleast 5 characters
// in a row that are not digits or special characters and ensures that there is
// atleast one vowel in the string.
//
// Args:
// s (string): The string to check.
//
// Returns:
// bool: True if the string likely contains words, false otherwise.
func LikelyContainsWords(s string) bool {
	if len(s) < 5 {
		return false
	}

	vowelCount := 0
	for i := 0; i < len(s)-4; i++ {
		if isWordLike(s[i : i+5]) {
			vowelCount++
		}
	}

	return vowelCount > 0
}

// isWordLike checks if a substring contains at least one vowel and no more
// than one digit or special character.
//
// Args:
// s (string): The substring to check.
//
// Returns:
// bool: True if the substring is likely a word, false otherwise.
func isWordLike(s string) bool {
	vowels := "aeiouAEIOU"
	digitOrSpecialCount := 0
	hasVowel := false

	for _, char := range s {
		if strings.ContainsRune(vowels, char) {
			hasVowel = true
		} else if !unicode.IsLetter(char) {
			digitOrSpecialCount++
		}
	}

	return hasVowel && digitOrSpecialCount <= 1
}

// IsQualityCandidateCheck checks if a string meets the quality regex or not
//
// Args:
// s (string): The string to check
//
// Returns:
// bool: True if the string is a quality candidate, false otherwise
func IsQualityCandidateCheck(s string) bool {
	// This regex checks for common patterns that are likely not quality
	// candidates.
	qualityRegex := `(xiaonei|zomato|fbobh|fccdbbcdaa|yahoo|linkedin|gmail|yandex|hotmail)|http:\\/\\/|https:\\/\\/|\\@.*\\.net|<tr>|<div>|<a href|<p>|<img src|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Za-z]{2,6}|^[0-9]+$|^.{0,5}$`
	re := regexp.MustCompile(qualityRegex)
	return !re.MatchString(s)
}
