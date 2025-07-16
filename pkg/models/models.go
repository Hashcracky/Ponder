// Package models contains the models for the application
package models

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// Config holds the configuration for the application
type Config struct {
	SourceDirectory string `json:"source_directory"`
	SourceWordlist  string `json:"source_wordlist"`
	WizardWordlist  string `json:"wizard_wordlist"`
}

// ConfigFilePath is the path to the configuration file
var ConfigFilePath = "/etc/ponder/config.json"

// SourceDirectory is the directory where the source wordlist is located
// Default is /data
var SourceDirectory = "/data"

// ImportDirectory is the directory where the import wordlist is located
// Default is /data/import
var ImportDirectory = fmt.Sprintf("%s/import", SourceDirectory)

// SourceWordlist is the path to the source wordlist
// Default is /data/source-wordlist.txt
var SourceWordlist = fmt.Sprintf("%s/source-wordlist.txt", SourceDirectory)

// WizardWordlist is the path to the wizard wordlist
// Default is /data/wizard-wordlist.txt
var WizardWordlist = fmt.Sprintf("%s/wizard-wordlist.txt", SourceDirectory)

// LastUpdated is the last time the wordlist was updated
var LastUpdated = time.Time{}

// LastUploaded is the last time data was uploaded
var LastUploaded = time.Time{}

// LogFile is the path to the log file
var LogFile = fmt.Sprintf("%s/log.txt", SourceDirectory)

// Log is used to track server-side events
type Log struct {
	Entries []LogEntry `json:"entries"`
}

// LogEntry is used to log an event to the log
type LogEntry struct {
	Time    string `json:"time"`
	Event   string `json:"event"`
	Message string `json:"message"`
}

// LoadConfig reads the configuration from a JSON file and assigns the values
// to the global variables.
//
// Args:
// None
//
// Returns:
// (*Config): The configuration object
// (error): Any error that occurred
func LoadConfig() (*Config, error) {
	file, err := os.Open(ConfigFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	// Assign the values to the global variables
	SourceDirectory = config.SourceDirectory
	SourceWordlist = config.SourceWordlist
	WizardWordlist = config.WizardWordlist

	return &config, nil
}

// GenerateNGramSliceBytes takes a byte slice and generates a new byte slice
// using the GenerateNGramsBytes function and combines the results.
// This function is used to generate n-grams from the input byte slice.
//
// Args:
// input ([]byte): The original byte slice to generate n-grams from
// wordRangeStart (int): The starting number of words to use for n-grams
// wordRangeEnd (int): The ending iteration number of words to use for n-grams
//
// Returns:
// ([]byte): A new byte slice with the n-grams generated
func GenerateNGramSliceBytes(input []byte, wordRangeStart int, wordRangeEnd int) []byte {
	data := string(input)
	lines := strings.Split(data, "\n")
	var newList []string

	for _, line := range lines {
		nGrams := GenerateNGrams(line, wordRangeStart, wordRangeEnd)
		newList = append(newList, nGrams...)
	}

	return []byte(strings.Join(newList, "\n"))
}

// GenerateNGrams generates n-grams from a string of text and returns a slice of n-grams
//
// Args:
// text (string): The text to generate n-grams from
// wordRangeStart (int): The starting number of words to use for n-grams
// wordRangeEnd (int): The ending iteration number of words to use for n-grams
//
// Returns:
// []string: A slice of n-grams
func GenerateNGrams(text string, wordRangeStart int, wordRangeEnd int) []string {
	words := strings.Fields(text)
	var nGrams []string

	for i := wordRangeStart; i <= wordRangeEnd; i++ {
		for j := 0; j <= len(words)-i; j++ {
			// Primary
			nGram := strings.Join(words[j:j+i], " ")
			nGram = strings.TrimSpace(nGram)
			nGram = strings.TrimLeft(nGram, " ")
			nGram = strings.ReplaceAll(nGram, ".", "")
			nGram = strings.ReplaceAll(nGram, ",", "")
			nGram = strings.ReplaceAll(nGram, ";", "")
			nGrams = append(nGrams, nGram)
		}
	}

	return nGrams
}

// EnforceLengthRange filters the input byte slice to only include strings
// between minLength and maxLength characters inclusive.
//
// Args:
// input ([]byte): The input byte slice to filter.
// minLength (int): The minimum length of strings to include.
// maxLength (int): The maximum length of strings to include.
//
// Returns:
// ([]byte): A new byte slice with strings within the specified length range.
func EnforceLengthRange(input []byte, minLength int, maxLength int) []byte {
	lines := strings.Split(string(input), "\n")
	var filtered []string

	for _, line := range lines {
		if len(line) >= minLength && len(line) <= maxLength {
			filtered = append(filtered, line)
		}
	}

	return []byte(strings.Join(filtered, "\n"))
}

// ConvertHexToPlaintext is a function that converts a "$HEX[plaintext]"
// string to a "plaintext" string.
//
// Args:
// hash: (string) the hash to convert
//
// Returns:
// plaintext: (string) the plaintext
// err: (error) any error that occurred
func ConvertHexToPlaintext(hash string) (string, error) {
	// Check if the hash is in the correct format
	hexPattern := regexp.MustCompile(`\$HEX\[(.*?)\]`)
	matches := hexPattern.FindAllStringSubmatch(hash, -1)
	if matches == nil {
		return hash, nil
	}

	var result strings.Builder
	lastIndex := 0

	for _, match := range matches {
		start := strings.Index(hash[lastIndex:], match[0]) + lastIndex
		end := start + len(match[0])

		// Append text before the $HEX[...] part
		result.WriteString(hash[lastIndex:start])

		// Convert the hex part to plaintext
		decodedPlaintext, err := hex.DecodeString(match[1])
		if err != nil {
			return "", err
		}

		// Append the decoded plaintext
		result.WriteString(string(decodedPlaintext))

		lastIndex = end
	}

	// Append any remaining text after the last $HEX[...] part
	result.WriteString(hash[lastIndex:])

	return result.String(), nil
}
