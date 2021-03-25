package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"golang.org/x/xerrors"
)

type Solution struct {
	Name   string `json:"name"`
	Result bool   `json:"result"`
}
type TaskInfo struct {
	Name      string     `json:"name"`
	Solutions []Solution `json:"solutions"`
}

func formatSolution(result bool) string {
	if result {
		return ":o:"
	} else {
		return ":x:"
	}
}

func run() error {
	var input string
	flag.StringVar(&input, "input", "", "json file to parse. blank or - mean to read from stdin")

	flag.Usage = func() {
		fmt.Printf("Usage: %s\n\n", input)
		flag.PrintDefaults()
	}
	flag.Parse()

	// たまたまシグネチャが同じだからといってこんなことをしてはいけない
	jsonBlob, err := (func() ([]byte, error) {
		if input == "" || input == "-" {
			return io.ReadAll(os.Stdin)
		} else {
			return os.ReadFile(input)
		}
	})()
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}
	var taskInfos []TaskInfo
	if err := json.Unmarshal(jsonBlob, &taskInfos); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	sb := strings.Builder{}
	sb.WriteString("| Task | Solution | Result |\n")
	sb.WriteString("| ---- | -------- | ------ |\n")
	for _, taskInfo := range taskInfos {
		if len(taskInfo.Solutions) == 0 {
			sb.WriteString(fmt.Sprintf("| %s |   :x:    |        |\n", taskInfo.Name))
		} else {
			for i, solution := range taskInfo.Solutions {
				if i == 0 {
					sb.WriteString(fmt.Sprintf(
						"| %s | %s | %s |\n",
						taskInfo.Name,
						solution.Name,
						formatSolution(solution.Result),
					))
				} else {
					sb.WriteString(fmt.Sprintf(
						"|    | %s | %s |\n",
						solution.Name,
						formatSolution(solution.Result),
					))
				}
			}
		}
	}

	fmt.Println(sb.String())
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
