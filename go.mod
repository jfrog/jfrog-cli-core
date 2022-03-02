module github.com/jfrog/jfrog-cli-core/v2

go 1.17

require (
)

// Exclude vulnerable dependencies
exclude (
	github.com/miekg/dns v1.0.14
	github.com/pkg/sftp v1.10.1
)

replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-client-go v1.10.1-0.20220301072905-2929c6d03263

// replace github.com/jfrog/build-info-go => github.com/jfrog/build-info-go v1.0.2-0.20220222144839-297d97db5248

// replace github.com/jfrog/gofrog => github.com/jfrog/gofrog v1.0.7-0.20211128152632-e218c460d703
