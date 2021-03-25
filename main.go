package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	yaml "github.com/goccy/go-yaml"
	"golang.org/x/xerrors"
)

type TaskYaml struct {
	Name string `yaml:"name"`
	Flag string `yaml:"flag"`
}
type Solution struct {
	Name   string `json:"name"`
	Result bool   `json:"result"`
}
type TaskInfo struct {
	Name      string     `json:"name"`
	Solutions []Solution `json:"solutions"`
}

const DefaultNewtork = `verify`
const DockerComposeOverride = `
version: '3'
networks:
  default:
    name: verify
`

func MakeOverride(dir string) (func(), error) {
	path := filepath.Join(dir, "docker-compose.override.yml")
	err := os.WriteFile(path, []byte(DockerComposeOverride), 0755)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	return func() {
		os.Remove(path)
	}, nil
}

func CheckTask(taskDir string, timeout time.Duration) (*TaskInfo, error) {
	log.Printf("Task: %s\n", taskDir)
	solutionDir := filepath.Join(taskDir, "solution")
	dockerCompose := filepath.Join(taskDir, "docker-compose.yml")

	yamlBlob, err := os.ReadFile(filepath.Join(taskDir, "task.yml"))
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	taskYaml := TaskYaml{}
	if err := yaml.Unmarshal(yamlBlob, &taskYaml); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	solutions, err := os.ReadDir(solutionDir)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	if _, err := os.Stat(dockerCompose); err == nil {
		// docker-compose.ymlが存在すれば、networkを起動してdocker-compose upする
		// deferで関数を抜けるときにdocker-compose downし、networkを削除する

		// docker-compose.override.yml でnetworkを指定する
		remove, _ := MakeOverride(taskDir)
		exec.Command("docker", "network", "create", DefaultNewtork)

		cmd := exec.Command("docker-compose", "up", "-d", "--build")
		cmd.Dir = taskDir
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		cmd.Run()

		defer func() {
			cmd := exec.Command("docker-compose", "down")
			cmd.Dir = taskDir
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
			cmd.Run()
			remove()
			exec.Command("docker", "network", "rm", DefaultNewtork)
		}()
	}

	// いくつかのsolutionがある予定
	results := make([]Solution, 0)
	for _, solution := range solutions {
		if !solution.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(solutionDir, solution.Name(), "solve.bash")); err != nil {
			continue
		}

		func() {
			log.Printf("Solution: %s\n", solution.Name())

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// docker-compose.override.yml でnetworkを指定する
			remove, _ := MakeOverride(filepath.Join(solutionDir, solution.Name()))
			defer remove()

			cmd := exec.CommandContext(ctx, "bash", "solve.bash")
			cmd.Dir = filepath.Join(solutionDir, solution.Name())
			cmd.Env = os.Environ()
			cmd.Env = append(cmd.Env, "HOST="+filepath.Base(taskDir))
			log.Printf("Env = %+v\n", cmd.Env)
			stdouterr, _ := cmd.CombinedOutput()
			log.Println(string(stdouterr))
			results = append(results, Solution{
				Name:   solution.Name(),
				Result: strings.Contains(string(stdouterr), taskYaml.Flag),
			})
		}()
	}
	return &TaskInfo{
		Name:      taskYaml.Name,
		Solutions: results,
	}, nil
}

func run() error {
	var dir string
	var timeout string
	var output string
	flag.StringVar(&dir, "dir", "", "directory to parse")
	flag.StringVar(&output, "output", "-", "a filename to output result")
	flag.StringVar(&timeout, "timeout", "10m", "timeout of solution running time")

	flag.Usage = func() {
		fmt.Printf("Usage: %s\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if _, err := os.Stat(dir); err != nil {
		return xerrors.Errorf("directory not found: %s", dir)
	}
	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	taskDirs := make([]string, 0)
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.Name() != "task.yml" {
			return nil
		}

		taskDir := filepath.Dir(path)
		solutionDir := filepath.Join(taskDir, "solution")
		if _, err := os.Stat(solutionDir); err == nil {
			taskDirs = append(taskDirs, taskDir)
		}
		return filepath.SkipDir
	})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	taskInfos := make([]*TaskInfo, len(taskDirs))
	for i, taskDir := range taskDirs {
		taskInfos[i], err = CheckTask(taskDir, timeoutDuration)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	jsonBlob, err := json.Marshal(taskInfos)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if output == "" || output == "-" {
		fmt.Println(string(jsonBlob))
	} else {
		if err := os.WriteFile(output, jsonBlob, 0644); err != nil {
			return xerrors.Errorf(": %w", err)
		}
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
