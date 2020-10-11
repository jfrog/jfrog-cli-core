# jfrog-cli-core

## General

**jfrog-cli-core** is a go module which contains the core code components used by the [JFrog CLI source code](https://github.com/jfrog-cli).

## Pull Requests

We welcome pull requests from the community.

### Guidelines

- If the existing tests do not already cover your changes, please add tests.
- Pull requests should be created on the **dev** branch.
- Please use gofmt for formatting the code before submitting the pull request.

# Tests

To run the tests, execute the following command from within the root directory of the project:

```sh
go test -v github.com/jfrog/jfrog-cli-core/tests -timeout 0
```
