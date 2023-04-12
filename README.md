| Branch |                                                                                                                                                                                            Status                                                                                                                                                                                            |
|:------:|:--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------:|
| master | [![Test](https://github.com/jfrog/jfrog-cli-core/actions/workflows/test.yml/badge.svg?branch=master)](https://github.com/jfrog/jfrog-cli-core/actions/workflows/test.yml?query=branch%3Amaster) [![Static Analysis](https://github.com/jfrog/jfrog-cli-core/actions/workflows/analysis.yml/badge.svg?branch=master)](https://github.com/jfrog/jfrog-cli-core/actions/workflows/analysis.yml) |
|  dev   |     [![Test](https://github.com/jfrog/jfrog-cli-core/actions/workflows/test.yml/badge.svg?branch=dev)](https://github.com/jfrog/jfrog-cli-core/actions/workflows/test.yml?query=branch%3Adev) [![Static Analysis](https://github.com/jfrog/jfrog-cli-core/actions/workflows/analysis.yml/badge.svg?branch=dev)](https://github.com/jfrog/jfrog-cli-core/actions/workflows/analysis.yml)      |
|        |     [![Scanned by Frogbot](https://raw.github.com/jfrog/frogbot/master/images/frogbot-badge.svg)](https://github.com/jfrog/frogbot#readme)                                                                                                                                                                                                                                                   |

# jfrog-cli-core

## General

**jfrog-cli-core** is a go module which contains the core code components used by the [JFrog CLI source code](https://github.com/jfrog/jfrog-cli).

## Pull Requests

We welcome pull requests from the community.

### Guidelines

- If the existing tests do not already cover your changes, please add tests.
- Pull requests should be created on the **dev** branch.
- Please use gofmt for formatting the code before submitting the pull request.

# Tests

To run the tests, execute the following command from within the root directory of the project:

```sh
go test -v github.com/jfrog/jfrog-cli-core/v2/tests -timeout 0
```
