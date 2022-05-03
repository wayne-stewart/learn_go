package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

func main() {
	csv_file_name := flag.String("csv", "problems.csv", "a csv file in the format of 'question,answer'")
	time_limit := flag.Int("limit", 30, "the time limit for the quiz in seconds")
	shuffle_flag := flag.Bool("shuffle", false, "specify if you want the problem list to be shuffled")
	flag.Parse()

	file, err := os.Open(*csv_file_name)
	if err != nil {
		exit(fmt.Sprintf("Failed to open the CSV file: %s\n", *csv_file_name))
	}

	reader := csv.NewReader(file)
	lines, err := reader.ReadAll()
	if err != nil {
		exit("Failed to parse the provided csv file.")
	}

	problems := parseLines(lines)

	if *shuffle_flag {
		shuffle(problems)
	}

	timer := time.NewTimer(time.Duration(*time_limit) * time.Second)
	correct := 0

	for i, p := range problems {
		fmt.Printf("Problem #%d: %s = ", i+1, p.q)
		a_ch := make(chan string)
		go func() {
			var answer string
			fmt.Scanf("%s\n", &answer)
			a_ch <- answer
		}()
		select {
		case <-timer.C:
			fmt.Printf("\nYou scored %d out of %d.\n", correct, len(problems))
			return
		case answer := <-a_ch:
			if answer == p.a {
				correct++
			}
		}
	}

	fmt.Printf("You scored %d out of %d.\n", correct, len(problems))
}

func parseLines(lines [][]string) []problem {
	ret := make([]problem, len(lines))
	for i, line := range lines {
		ret[i] = problem{
			q: line[0],
			a: strings.TrimSpace(line[1]),
		}
	}
	return ret
}

func shuffle(problems []problem) {
	rand.Seed(time.Now().UnixNano())
	min := 0
	max := len(problems)
	for i := 0; i < max; i++ {
		j := rand.Intn(max-min) + min
		swap(problems, i, j)
	}
}

func swap(problems []problem, i int, j int) {
	t := problems[i]
	problems[i] = problems[j]
	problems[j] = t
}

type problem struct {
	q string
	a string
}

func exit(msg string) {
	fmt.Println(msg)
	os.Exit(1)
}
