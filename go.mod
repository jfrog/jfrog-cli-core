module github.com/jfrog/jfrog-cli-core

go 1.14

require (
	github.com/buger/jsonparser v0.0.0-20180910192245-6acdf747ae99
	github.com/c-bata/go-prompt v0.2.5
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e
	github.com/codegangsta/cli v1.20.0
	github.com/jfrog/gocmd v0.2.0
	github.com/jfrog/gofrog v1.0.6
	github.com/jfrog/jfrog-client-go v0.22.3
	github.com/magiconair/properties v1.8.1
	github.com/manifoldco/promptui v0.8.0
	github.com/mattn/go-shellwords v1.0.3
	github.com/pkg/errors v0.9.1
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.6.1
	golang.org/x/crypto v0.0.0-20201208171446-5f87f3452ae9
	golang.org/x/mod v0.3.0
	gopkg.in/yaml.v2 v2.3.0
)

replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-client-go v0.22.4-0.20210519160047-2add626ec176

// replace github.com/jfrog/gocmd => github.com/jfrog/gocmd v0.1.20-0.20210330134550-459be3fc0ee8
