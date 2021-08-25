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
	"time"

	yaml "github.com/goccy/go-yaml"
	"github.com/golang/glog"
	dockercompose "github.com/theoremoon/verify-tasks/docker-compose"
	"github.com/theoremoon/verify-tasks/types"
	"github.com/theoremoon/verify-tasks/verifier"
	"golang.org/x/mod/sumdb/dirhash"
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
type VerifyJson struct {
	Results []TaskInfo        `json:"results"`
	Hashes  map[string]string `json:"hashes"`
}

func GetDirHash(taskDir string) (string, string, error) {
	yamlBlob, err := os.ReadFile(filepath.Join(taskDir, "task.yml"))
	if err != nil {
		return "", "", xerrors.Errorf(": %w", err)
	}

	taskYaml := types.TaskYaml{}
	if err := yaml.Unmarshal(yamlBlob, &taskYaml); err != nil {
		return "", "", xerrors.Errorf(": %w", err)
	}

	h1, err := dirhash.HashDir(taskDir, "", dirhash.Hash1)
	if err != nil {
		return "", "", xerrors.Errorf(": %w", err)
	}
	return taskYaml.Name, h1, nil
}

func CheckTask(taskDir string, timeout time.Duration) (*TaskInfo, error) {
	log.Printf("Task: %s\n", taskDir)
	solutionDir := filepath.Join(taskDir, "solution")
	dockerCompose := filepath.Join(taskDir, "docker-compose.yml")

	yamlBlob, err := os.ReadFile(filepath.Join(taskDir, "task.yml"))
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	taskInfo := types.TaskYaml{}
	if err := yaml.Unmarshal(yamlBlob, &taskInfo); err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	solutions, err := os.ReadDir(solutionDir)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	// どうせdocker-compose downで消えるのでこのあたりで作って消す
	exec.Command("docker", "network", "create", dockercompose.DefaultNewtork).Run()
	defer func() {
		exec.Command("docker", "network", "rm", dockercompose.DefaultNewtork).Run()
	}()

	if _, err := os.Stat(dockerCompose); err == nil {
		// docker-compose.ymlが存在するとき必要な変数を定義させる
		if taskInfo.Main == nil || taskInfo.LocalPort == nil {
			glog.Errorln("`main` and `localport` are required in task.yml")
			return &TaskInfo{
				Name:      taskInfo.Name,
				Solutions: []Solution{},
			}, nil
		}

		// docker-compose.ymlが存在すれば立ち上げる
		c := dockercompose.New(taskDir, dockercompose.DefaultNewtork)
		if err := c.Setup(context.Background()); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}

		defer func() {
			c.Teardown(context.Background())
		}()
	}

	// いくつかのsolutionがある予定
	results := make([]Solution, 0)
	for _, solution := range solutions {
		if !solution.IsDir() {
			continue
		}
		solveDockerfile := filepath.Join(solutionDir, solution.Name(), "Dockerfile")
		var v verifier.Verifier
		if _, err := os.Stat(solveDockerfile); err == nil {
			v = verifier.NewDocker(taskDir, filepath.Join(solutionDir, solution.Name()), dockercompose.DefaultNewtork, taskInfo)
		}
		if v == nil {
			glog.Errorln("no solution verifier is found")
			results = append(results, Solution{
				Name:   solution.Name(),
				Result: false,
			})
			continue
		}

		// 全体でtimeoutだけ待つ
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := v.Setup(ctx); err != nil {
			glog.Infof("%+v\n", err)
			continue
		}
		defer v.Teardown(ctx)

		ok, err := v.Verify(ctx)
		if err != nil {
			glog.Infof("%+v\n", err)
			continue
		}
		results = append(results, Solution{
			Name:   solution.Name(), // directoryの名前
			Result: ok,
		})

	}
	return &TaskInfo{
		Name:      taskInfo.Name,
		Solutions: results,
	}, nil
}

func run() error {
	var dir string
	var timeout string
	var output string
	flag.StringVar(&dir, "dir", "", "directory to parse")
	flag.StringVar(&output, "json", "", "a json file to store the result")
	flag.StringVar(&timeout, "timeout", "10m", "timeout of solution running time")

	flag.Usage = func() {
		fmt.Printf("Usage: %s\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if _, err := os.Stat(dir); err != nil {
		return xerrors.Errorf("directory not found: %s", dir)
	}
	if output == "" {
		return xerrors.Errorf("output is not specified")
	}
	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	dataStore := VerifyJson{
		Results: []TaskInfo{},
		Hashes:  make(map[string]string),
	}
	if _, err := os.Stat(output); err == nil {
		blob, err := os.ReadFile(output)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		if err := json.Unmarshal(blob, &dataStore); err != nil {
			return xerrors.Errorf(": %w", err)
		}
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
		taskName, newHash, err := GetDirHash(taskDir)
		if err == nil {
			// ディレクトリのハッシュが以前と同じならその問題はスキップする
			pastHash, exist := dataStore.Hashes[taskName]
			if exist && pastHash == newHash {
				log.Printf("Skip: %s\n", taskName)
				return nil
			}

			dataStore.Hashes[taskName] = newHash
		}

		solutionDir := filepath.Join(taskDir, "solution")
		if _, err := os.Stat(solutionDir); err == nil {
			taskDirs = append(taskDirs, taskDir)
		}
		return filepath.SkipDir
	})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	for _, taskDir := range taskDirs {
		tInfo, err := CheckTask(taskDir, timeoutDuration)
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}

		found := false
		for i, v := range dataStore.Results {
			if v.Name == tInfo.Name {
				dataStore.Results[i] = *tInfo
				found = true
				break
			}
		}

		if !found {
			dataStore.Results = append(dataStore.Results, *tInfo)
		}
	}
	jsonBlob, err := json.MarshalIndent(dataStore, "", "  ")
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	if err := os.WriteFile(output, jsonBlob, 0644); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
	glog.Flush()
}
