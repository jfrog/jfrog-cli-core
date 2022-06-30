module github.com/jfrog/jfrog-cli-core/v2

go 1.17

require (
	github.com/buger/jsonparser v1.1.1
	github.com/c-bata/go-prompt v0.2.5 // Should not be updated to 0.2.6 due to a bug (https://github.com/jfrog/jfrog-cli-core/pull/372)
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e
	github.com/forPelevin/gomoji v1.1.4
	github.com/gocarina/gocsv v0.0.0-20220531201732-5f969b02b902
	github.com/google/uuid v1.3.0
	github.com/gookit/color v1.5.1
	github.com/jedib0t/go-pretty/v6 v6.3.2
	github.com/jfrog/build-info-go v1.3.0
	github.com/jfrog/gofrog v1.1.2
	github.com/jfrog/jfrog-client-go v1.14.0
	github.com/magiconair/properties v1.8.5
	github.com/manifoldco/promptui v0.9.0
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8
	github.com/pkg/errors v0.9.1
	github.com/spf13/viper v1.10.1
	github.com/stretchr/testify v1.7.2
	github.com/urfave/cli v1.22.9
	github.com/vbauerster/mpb/v7 v7.4.2
	golang.org/x/mod v0.5.1
	golang.org/x/term v0.0.0-20220526004731-065cf7ba2467
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/CycloneDX/cyclonedx-go v0.5.1 // indirect
	github.com/Microsoft/go-winio v0.5.0 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20210428141323-04723f9f07d7 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/acomagu/bufpipe v1.0.3 // indirect
	github.com/andybalholm/brotli v1.0.3 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dsnet/compress v0.0.2-0.20210315054119-f66993602bf5 // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.3.1 // indirect
	github.com/go-git/go-git/v5 v5.4.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.1 // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v0.0.0-20201106050909-4977a11b4351 // indirect
	github.com/klauspost/compress v1.13.5 // indirect
	github.com/klauspost/cpuid/v2 v2.0.6 // indirect
	github.com/klauspost/pgzip v1.2.5 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/mattn/go-tty v0.0.3 // indirect
	github.com/mholt/archiver/v3 v3.5.1 // indirect
	github.com/minio/sha256-simd v1.0.1-0.20210617151322-99e45fae3395 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/nwaples/rardecode v1.1.0 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/pierrec/lz4/v4 v4.1.2 // indirect
	github.com/pkg/term v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/ulikunitz/xz v0.5.9 // indirect
	github.com/xanzy/ssh-agent v0.3.1 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/xo/terminfo v0.0.0-20210125001918-ca9a967f8778 // indirect
	golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e // indirect
	golang.org/x/net v0.0.0-20211112202133-69e39bad7dc2 // indirect
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gopkg.in/ini.v1 v1.66.2 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Exclude vulnerable dependencies
exclude (
	github.com/miekg/dns v1.0.14
	github.com/pkg/sftp v1.10.1
)

replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-client-go v1.14.1-0.20220630090055-53a8c69a5f05

replace github.com/jfrog/build-info-go => github.com/jfrog/build-info-go v1.3.1-0.20220623084021-a61432b84f63

replace github.com/jfrog/gofrog => github.com/jfrog/gofrog v1.1.3-0.20220628060849-fcdbffb153da
