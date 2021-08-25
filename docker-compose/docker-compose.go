package dockercompose

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/xerrors"
)

const DefaultNewtork = `verify`
const DockerComposeOverride = `
version: '3'
networks:
  default:
    name: %s
`

type Compose interface {
	Setup(context.Context) error
	Teardown(context.Context) error
}

type compose struct {
	dir      string
	override string
	network  string
}

func New(dir string, network string) Compose {
	return &compose{
		dir:     dir,
		network: network,
	}
}

func (c *compose) Setup(ctx context.Context) error {
	c.override = filepath.Join(c.dir, "docker-compose.override.yml")
	content := fmt.Sprintf(DockerComposeOverride, c.network)
	err := os.WriteFile(c.override, []byte(content), 0755)
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	cmd := exec.Command("docker-compose", "up", "--build", "-d") // -f を指定しないことで暗黙にdocker-compose.override.ymlを読み込ませる
	cmd.Dir = c.dir
	if err := cmd.Run(); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	return nil
}

func (c *compose) Teardown(ctx context.Context) error {
	defer func() {
		os.Remove(c.override)
	}()
	cmd := exec.Command("docker-compose", "down")
	cmd.Dir = c.dir
	if err := cmd.Run(); err != nil {
		return xerrors.Errorf(": %w", err)
	}
	return nil
}
