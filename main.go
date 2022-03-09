package main

import (
	"bufio"
	"bytes"
	"embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	homedir "github.com/mitchellh/go-homedir"
)

//go:embed words.txt
var f embed.FS

// guess represents a given guess. The word that was guessed and the hits, or
// results, for the given word.
type guess struct {
	word string
	hits string
}

var reGuessesFile string
var reGuessesDir string

func init() {
	currentDate := time.Now()

	home, err := homedir.Dir()
	if err != nil {
		fmt.Println("Bad error: ", err)
		return
	}
	var (
		defaultDir  = fmt.Sprintf("%s/.re", home)
		defaultFile = fmt.Sprintf("%s/%s-guesses.txt", defaultDir, currentDate.Format("2006-01-02"))
		usage       = "specify a guesses.txt filepath to persist your daily guesses"
	)
	flag.StringVar(&reGuessesFile, "file", defaultFile, usage)
	flag.StringVar(&reGuessesFile, "f", defaultFile, usage+" (shorthand)")

	reGuessesDir = defaultDir
}

func main() {
	flag.Parse()

	if _, err := os.Stat(reGuessesDir); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(reGuessesDir, os.ModePerm)
		if err != nil {
			fmt.Println("err: ", err)
			os.Exit(1)
		}
	}

	var file os.File
	var guesses = make(map[int]guess)

	if _, err := os.Stat(reGuessesFile); errors.Is(err, os.ErrNotExist) {
		rf, err := os.Create(reGuessesFile)
		if err != nil {
			fmt.Println("err: ", err)
			os.Exit(1)
		}
		file = *rf
	} else {
		rf, err := os.OpenFile(reGuessesFile, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0655)
		if err != nil {
			fmt.Println("err: ", err)
			os.Exit(1)
		}
		file = *rf

		guessCount := 0
		count := 0

		guessWord := ""
		hitWord := ""

		scanner := bufio.NewScanner(&file)
		for scanner.Scan() {
			if count == 0 {
				guessWord = scanner.Text()
				count = 1
			} else if count == 1 {
				hitWord = scanner.Text()
				count = 0
				guesses[guessCount] = guess{
					word: guessWord,
					hits: hitWord,
				}
				guessCount++
			}
		}

		if err := scanner.Err(); err != nil {
			fmt.Println("err: ", err)
			os.Exit(1)
		}
	}

	defer file.Close()

	if len(guesses) > 0 && len(guesses) < 5 {
		fmt.Println("== Already guessed ==")
		for _, g := range guesses {
			fmt.Printf("%s :: %s\n", g.word, g.hits)
		}
	} else {
		if len(guesses) == 5 {
			fmt.Println("You are out of guesses... Better luck tomorrow!")
			os.Exit(0)
		}
	}

	words, err := loadWords()
	if err != nil {
		fmt.Println("err: ", err)
		os.Exit(1)
	}

	validateGuess := func(input string) error {
		if len(input) != 5 {
			return errors.New("Invalid string, input length is 5")
		}

		return nil
	}

	validateHits := func(input string) error {
		if len(input) != 5 {
			return errors.New("Invalid string, input length is 5")
		}

		return nil
	}

	templates := &promptui.PromptTemplates{
		Prompt:  "{{ . }} ",
		Valid:   "{{ . | green }} ",
		Invalid: "{{ . | red }} ",
		Success: "{{ . | bold }} ",
	}

	w := bufio.NewWriter(&file)

	for i := len(guesses); i < 5; i++ {
		prompt := promptui.Prompt{
			Label:     "Guess (xxxxx)",
			Validate:  validateGuess,
			Templates: templates,
		}
		guessResult, err := prompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		hitsPrompt := promptui.Prompt{
			Label:     "Hit letter (format example: gy..g)",
			Validate:  validateHits,
			Templates: templates,
		}
		hitsResult, err := hitsPrompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		guesses[i] = guess{
			word: guessResult,
			hits: hitsResult,
		}

		_, err = w.WriteString(fmt.Sprintf("%s\n", guessResult))
		if err != nil {
			fmt.Println("err: ", err)
			os.Exit(1)
		}
		_, err = w.WriteString(fmt.Sprintf("%s\n", hitsResult))
		if err != nil {
			fmt.Println("err: ", err)
			os.Exit(1)
		}

		if i != 4 {
			donePrompt := promptui.Prompt{
				Label:     "Add another guess?",
				IsConfirm: true,
			}
			_, err = donePrompt.Run()
			if err == promptui.ErrAbort {
				break
			} else if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				return
			}
		}
	}

	w.Flush()

	nogo := getNogos(guesses)
	wordsWithoutNogo := removeNogoLetters(words, nogo)

	green := getGreens(guesses)
	greenWords := removeMissingGreenMatches(wordsWithoutNogo, green)

	yellows := getYellows(guesses)
	finalWords := removeYellowMatches(greenWords, yellows)

	fmt.Println("Possible words:")
	for i, word := range finalWords {
		if i > 99 {
			fmt.Println("... there are more than 100 words")
			break
		}
		fmt.Printf("  %s\n", word)
	}
}

// loadWords will load the words from the embedded file 'words.txt'.
func loadWords() ([]string, error) {
	data, err := f.ReadFile("words.txt")
	if err != nil {
		return []string{}, err
	}
	reader := bytes.NewReader(data)

	var words []string

	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanWords)

	for scanner.Scan() {
		words = append(words, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return []string{}, err
	}

	return words, nil
}

// removeNogoLetters will remove the words from the 's' slice that contain any
// characters from the 'nogo' slice.
func removeNogoLetters(s, nogo []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, word := range s {
		goodWord := true
		for _, n := range nogo {
			// Ignore word that contains a 'nogo' letter.
			if strings.Contains(word, n) {
				goodWord = false
				break
			}
		}
		if goodWord {
			if _, value := allKeys[word]; !value {
				allKeys[word] = true
				list = append(list, word)
			}
		}
	}
	return list
}

// removeMissingGreenMatches will remove words that do not include the green
// letters that have been guessed.
func removeMissingGreenMatches(s []string, green map[int]string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for j := 0; j < len(s); j++ {
		word := s[j]
		goodWord := true
		for i, c := range green {
			if c != "-" {
				if string(word[i]) != c {
					// Ignore the words that do not have a green letter at the
					// same position as the incoming green slice.
					goodWord = false
				}
			}
		}
		if goodWord {
			if _, value := allKeys[word]; !value {
				allKeys[word] = true
				list = append(list, word)
			}
		}
	}
	return list
}

// removeYellowMatches will remove words that have matches at the 'yellow' or
// nearby letters at the given guessed location.
func removeYellowMatches(s []string, yellow []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for j := 0; j < len(s); j++ {
		word := s[j]
		goodWord := true
		for i, c := range yellow {
			if c != "-" {
				for _, v := range c {
					if strings.Contains(word, string(v)) {
						if string(word[i]) == string(v) {
							goodWord = false
							break
						}
					} else {
						goodWord = false
						break
					}
				}
			}
		}
		if goodWord {
			if _, value := allKeys[word]; !value {
				allKeys[word] = true
				list = append(list, word)
			}
		}
	}
	return list
}

// getNogos will return the nogo letters from the guesses.
func getNogos(guesses map[int]guess) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for i := 0; i < len(guesses); i++ {
		guess := guesses[i].word
		score := guesses[i].hits
		for j := 0; j < 5; j++ {
			if string(score[j]) == "." {
				c := fmt.Sprintf("%c", guess[j])
				if _, value := allKeys[c]; !value {
					allKeys[c] = true
					list = append(list, c)
				}
			}
		}
	}
	return list
}

// getGreens will return an array of green letter matches from the guesses.
func getGreens(guesses map[int]guess) map[int]string {
	greenMap := make(map[int]string)
	green := []string{"-", "-", "-", "-", "-"}
	for i := 0; i < len(guesses); i++ {
		guess := guesses[i].word
		score := guesses[i].hits
		for k := range score {
			if string(score[k]) == "g" {
				c := fmt.Sprintf("%c", guess[k])
				green[k] = c
			}
		}
	}
	for i, v := range green {
		greenMap[i] = v
	}
	return greenMap
}

// getYellows will return an array of yellow letter matches from the guesses.
// The format will include all 'yellow' or 'y' matches for each location. You
// can have multiple 'y' matches in each location.
func getYellows(guesses map[int]guess) []string {
	yellow := []string{"", "", "", "", ""}
	for i := 0; i < len(guesses); i++ {
		guess := guesses[i].word
		score := guesses[i].hits
		for j := 0; j < 5; j++ {
			if string(score[j]) == "y" {
				c := fmt.Sprintf("%c", guess[j])
				// If there was already a yellow guess for character 'c' at
				// position 'j' we can ignore it.
				if !strings.Contains(yellow[j], c) {
					yellow[j] += c
				}
			}
		}
	}

	// If the list of yellow matches is empty, fill it with a '-' to denote
	// there was not a yellow match in the guesses.
	for i := 0; i < 5; i++ {
		if yellow[i] == "" {
			yellow[i] = "-"
		}
	}

	return yellow
}
