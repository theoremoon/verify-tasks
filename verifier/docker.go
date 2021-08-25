package verifier

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/google/uuid"
	"github.com/theoremoon/verify-tasks/types"
	"golang.org/x/xerrors"
)

type dockerVerifier struct {
	imageTag    string
	taskDir     string
	solutionDir string
	taskInfo    types.TaskYaml
	network     string
}

func NewDocker(taskDir, solutionDir string, network string, taskInfo types.TaskYaml) Verifier {
	return &dockerVerifier{
		taskDir:     taskDir,
		solutionDir: solutionDir,
		taskInfo:    taskInfo,
		network:     network,
	}
}

/// Docker build
func (v *dockerVerifier) Setup(ctx context.Context) error {
	v.imageTag = uuid.New().String()

	// build image
	build := exec.CommandContext(ctx, "docker", "build", ".", "-t", v.imageTag)
	build.Dir = v.solutionDir
	buildResult, err := build.CombinedOutput()
	if glog.V(2) || err != nil {
		glog.Infoln(string(buildResult))
	}

	return nil
}

func (v *dockerVerifier) Verify(ctx context.Context) (bool, error) {
	containerName := uuid.New().String()
	dir, _ := filepath.Abs(filepath.Join(v.taskDir, "distfiles"))

	host := "DUMMY=a"
	if v.taskInfo.Main != nil {
		host = fmt.Sprintf("HOST=%s", *v.taskInfo.Main)
	}

	port := "DUMMY=a"
	if v.taskInfo.LocalPort != nil {
		port = fmt.Sprintf("PORT=%d", *v.taskInfo.LocalPort)
	}

	run := exec.CommandContext(ctx,
		"docker", "run",
		"--name", containerName,
		"--rm",
		"--network", v.network,
		"-e", host,
		"-e", fmt.Sprintf("HOST-SELF=%s", containerName),
		"-e", port,
		"-v", dir+":/distfiles",
		v.imageTag,
	)
	run.Dir = filepath.Join(v.taskDir)

	stdouterr, _ := run.CombinedOutput()
	if glog.V(2) {
		glog.Infoln(string(stdouterr))
	}

	return strings.Contains(string(stdouterr), v.taskInfo.Flag), nil
}

func (v *dockerVerifier) Teardown(ctx context.Context) error {
	rm := exec.CommandContext(ctx, "docker", "rmi", v.imageTag)
	if err := rm.Run(); err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}
