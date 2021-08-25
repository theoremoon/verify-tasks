package types

type TaskYaml struct {
	Name string `yaml:"name"`
	Flag string `yaml:"flag"`

	Main      *string `yaml:"main"`
	LocalPort *int    `yaml:"local-port"`
}
