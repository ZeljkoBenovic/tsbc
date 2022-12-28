package sbc

import "fmt"

type Config struct {
	DomainName, Port string
}

type ISBC interface {
	Run()
}

type SBC struct {
	config Config
}

func NewSBC(sbcConfig Config) ISBC {
	return &SBC{
		config: sbcConfig,
	}
}

func (s *SBC) Run() {
	fmt.Printf("%#v\n", s)
}