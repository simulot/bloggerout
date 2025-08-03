module bloggerout

go 1.24

toolchain go1.24.1

require (
	github.com/JohannesKaufmann/dom v0.2.0
	github.com/simulot/TakeoutLocalization v0.1.5
	github.com/spf13/cobra v1.9.1
	golang.org/x/net v0.42.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/simulot/TakeoutLocalization => /home/jfcassan/Dev/translatetakeout

require (
	github.com/JohannesKaufmann/html-to-markdown/v2 v2.3.3
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
)
