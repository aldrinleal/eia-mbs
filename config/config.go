package config

type Config struct {
	GraderFunc string `yaml:"graderFunc"`
	SourcerCmd string `yaml:"sourcerCmd"`
	ServiceUrl string `yaml:"serviceUrl"`
	ListenAddr string `yaml:"listenAddr"`
}
