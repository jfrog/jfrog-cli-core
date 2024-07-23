package commandssummaries

import (
	"encoding/json"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBuildInfoTable(t *testing.T) {
	gh := &BuildInfoSummary{}
	var builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: "http://myJFrogPlatform/builds/buildName/123",
		},
	}
	expected := "\n\n|  Build Info |  Time Stamp | \n|---------|------------| \n| [buildName 123](http://myJFrogPlatform/builds/buildName/123) | May 5, 2024 , 12:47:20 |\n\n\n"
	assert.Equal(t, expected, gh.buildInfoTable(builds))
}

func TestBuildInfoModules(t *testing.T) {
	gh := &BuildInfoSummary{}
	var builds = []*buildinfo.BuildInfo{{}}
	err := json.Unmarshal([]byte(`{
    "properties" : {
      "buildInfo.env.ANDROID_SDK_ROOT" : "/usr/local/lib/android/sdk",
      "buildInfo.env.GITHUB_ACTION_REPOSITORY" : "",
      "buildInfo.env.DEBIAN_FRONTEND" : "noninteractive",
      "buildInfo.env.STATS_BLT" : "true",
      "buildInfo.env.RUNNER_TOOL_CACHE" : "/opt/hostedtoolcache",
      "buildInfo.env.ANDROID_NDK_ROOT" : "/usr/local/lib/android/sdk/ndk/25.2.9519653",
      "buildInfo.env.GITHUB_REF" : "refs/heads/master-build",
      "buildInfo.env.PWD" : "/home/runner/work/project-examples/project-examples",
      "buildInfo.env.LEIN_HOME" : "/usr/local/lib/lein",
      "buildInfo.env.GITHUB_ACTOR" : "RobiNino",
      "buildInfo.env.ANDROID_NDK_HOME" : "/usr/local/lib/android/sdk/ndk/25.2.9519653",
      "buildInfo.env.ACCEPT_EULA" : "Y",
      "buildInfo.env.JFROG_CLI_COMMAND_SUMMARY_OUTPUT_DIR" : "/home/runner/work/_temp",
      "buildInfo.env.GITHUB_REF_NAME" : "master-build",
      "buildInfo.env.ImageVersion" : "20240714.1.0",
      "buildInfo.env.STATS_D" : "true",
      "buildInfo.env.GITHUB_WORKFLOW_REF" : "RobiNino/project-examples/.github/workflows/masterbuild.yml@refs/heads/master-build",
      "buildInfo.env.GHCUP_INSTALL_BASE_PREFIX" : "/usr/local",
      "buildInfo.env.GITHUB_PATH" : "/home/runner/work/_temp/_runner_file_commands/add_path_824b4854-4b0e-4727-9f69-22315131a685",
      "buildInfo.env.GITHUB_BASE_REF" : "",
      "buildInfo.env.GITHUB_ACTION_REF" : "",
      "buildInfo.env.GOROOT_1_20_X64" : "/opt/hostedtoolcache/go/1.20.14/x64",
      "buildInfo.env.GITHUB_RUN_ID" : "10044646858",
      "buildInfo.env.DEPLOYMENT_BASEPATH" : "/opt/runner",
      "buildInfo.env.STATS_VMD" : "true",
      "buildInfo.env.GITHUB_REPOSITORY_OWNER" : "RobiNino",
      "buildInfo.env.GITHUB_RUN_NUMBER" : "10",
      "buildInfo.env.PERFLOG_LOCATION_SETTING" : "RUNNER_PERFLOG",
      "buildInfo.env.RUNNER_PERFLOG" : "/home/runner/perflog",
      "buildInfo.env.STATS_RDCL" : "true",
      "buildInfo.env.GITHUB_WORKFLOW" : "Master Build",
      "buildInfo.env.GITHUB_WORKFLOW_SHA" : "598306cba716e5f08c1acf6e650172dbb19cd68c",
      "buildInfo.env.JFROG_CLI_BUILD_NAME" : "Master Build",
      "buildInfo.env.GOROOT_1_22_X64" : "/opt/hostedtoolcache/go/1.22.5/x64",
      "buildInfo.env.JAVA_HOME_11_X64" : "/usr/lib/jvm/temurin-11-jdk-amd64",
      "buildInfo.env.STATS_TRP" : "true",
      "buildInfo.env.GITHUB_REPOSITORY_ID" : "160498980",
      "buildInfo.env.pythonLocation" : "/opt/hostedtoolcache/Python/3.11.5/x64",
      "buildInfo.env.INVOCATION_ID" : "14d9de2fedd94c199ffd998ff5ee6f9d",
      "buildInfo.env.JAVA_HOME_21_X64" : "/usr/lib/jvm/temurin-21-jdk-amd64",
      "buildInfo.env.STATS_VMFE" : "true",
      "buildInfo.env.JOURNAL_STREAM" : "8:1860",
      "buildInfo.env.CONDA" : "/usr/share/miniconda",
      "buildInfo.env.Python_ROOT_DIR" : "/opt/hostedtoolcache/Python/3.11.5/x64",
      "buildInfo.env.GITHUB_SHA" : "598306cba716e5f08c1acf6e650172dbb19cd68c",
      "buildInfo.env.NVM_DIR" : "/home/runner/.nvm",
      "buildInfo.env.Python3_ROOT_DIR" : "/opt/hostedtoolcache/Python/3.11.5/x64",
      "buildInfo.env.GITHUB_ACTION" : "__run_9",
      "buildInfo.env.AGENT_TOOLSDIRECTORY" : "/opt/hostedtoolcache",
      "buildInfo.env.GITHUB_RUN_ATTEMPT" : "1",
      "buildInfo.env.AZURE_EXTENSION_DIR" : "/opt/az/azcliextensions",
      "buildInfo.env.BOOTSTRAP_HASKELL_NONINTERACTIVE" : "1",
      "buildInfo.env.JFROG_CLI_BUILD_URL" : "https://github.com/RobiNino/project-examples/actions/runs/10044646858",
      "buildInfo.env.GITHUB_EVENT_PATH" : "/home/runner/work/_temp/_github_workflow/event.json",
      "buildInfo.env.GRADLE_HOME" : "/usr/share/gradle-8.9",
      "buildInfo.env.JFROG_CLI_OFFER_CONFIG" : "false",
      "buildInfo.env.CHROME_BIN" : "/usr/bin/google-chrome",
      "buildInfo.env.GITHUB_JOB" : "build",
      "buildInfo.env.SYSTEMD_EXEC_PID" : "590",
      "buildInfo.env.GITHUB_STEP_SUMMARY" : "/home/runner/work/_temp/_runner_file_commands/step_summary_824b4854-4b0e-4727-9f69-22315131a685",
      "buildInfo.env.RUNNER_ARCH" : "X64",
      "buildInfo.env.JAVA_HOME" : "/usr/lib/jvm/temurin-11-jdk-amd64",
      "buildInfo.env.VCPKG_INSTALLATION_ROOT" : "/usr/local/share/vcpkg",
      "buildInfo.env.GITHUB_TRIGGERING_ACTOR" : "RobiNino",
      "buildInfo.env.LANG" : "C.UTF-8",
      "buildInfo.env.RUNNER_OS" : "Linux",
      "buildInfo.env.SHLVL" : "1",
      "buildInfo.env.Python2_ROOT_DIR" : "/opt/hostedtoolcache/Python/3.11.5/x64",
      "buildInfo.env.XDG_RUNTIME_DIR" : "/run/user/1001",
      "buildInfo.env.GITHUB_REPOSITORY_OWNER_ID" : "43318887",
      "buildInfo.env.PKG_CONFIG_PATH" : "/opt/hostedtoolcache/Python/3.11.5/x64/lib/pkgconfig",
      "buildInfo.env.CI" : "true",
      "buildInfo.env.ImageOS" : "ubuntu22",
      "buildInfo.env.JAVA_HOME_17_X64" : "/usr/lib/jvm/temurin-17-jdk-amd64",
      "buildInfo.env.GITHUB_WORKSPACE" : "/home/runner/work/project-examples/project-examples",
      "buildInfo.env.JFROG_CLI_USER_AGENT" : "setup-jfrog-cli-github-action/4.1.3",
      "buildInfo.env.LD_LIBRARY_PATH" : "/opt/hostedtoolcache/Python/3.11.5/x64/lib",
      "buildInfo.env.ANDROID_HOME" : "/usr/local/lib/android/sdk",
      "buildInfo.env.CHROMEWEBDRIVER" : "/usr/local/share/chromedriver-linux64",
      "buildInfo.env.ACTIONS_RUNNER_ACTION_ARCHIVE_CACHE" : "/opt/actionarchivecache",
      "buildInfo.env.GOROOT_1_21_X64" : "/opt/hostedtoolcache/go/1.21.12/x64",
      "buildInfo.env.JFROG_CLI_ENV_EXCLUDE" : "*password*;*secret*;*key*;*token*;*auth*;JF_ARTIFACTORY_*;JF_ENV_*;JF_URL;JF_USER;JF_PASSWORD;JF_ACCESS_TOKEN",
      "buildInfo.env.GITHUB_ENV" : "/home/runner/work/_temp/_runner_file_commands/set_env_824b4854-4b0e-4727-9f69-22315131a685",
      "buildInfo.env.GITHUB_REPOSITORY" : "RobiNino/project-examples",
      "buildInfo.env.RUNNER_ENVIRONMENT" : "github-hosted",
      "buildInfo.env.GITHUB_SERVER_URL" : "https://github.com",
      "buildInfo.env.GITHUB_STATE" : "/home/runner/work/_temp/_runner_file_commands/save_state_824b4854-4b0e-4727-9f69-22315131a685",
      "buildInfo.env.SWIFT_PATH" : "/usr/share/swift/usr/bin",
      "buildInfo.env.ANDROID_NDK" : "/usr/local/lib/android/sdk/ndk/25.2.9519653",
      "buildInfo.env.ANT_HOME" : "/usr/share/ant",
      "buildInfo.env.GITHUB_ACTOR_ID" : "43318887",
      "buildInfo.env.GITHUB_REF_TYPE" : "branch",
      "buildInfo.env.GITHUB_OUTPUT" : "/home/runner/work/_temp/_runner_file_commands/set_output_824b4854-4b0e-4727-9f69-22315131a685",
      "buildInfo.env.RUNNER_NAME" : "GitHub Actions 8",
      "buildInfo.env.GITHUB_API_URL" : "https://api.github.com",
      "buildInfo.env.PATH" : "/opt/hostedtoolcache/node/16.20.2/x64/bin:/opt/hostedtoolcache/Python/3.11.5/x64/bin:/opt/hostedtoolcache/Python/3.11.5/x64:/opt/hostedtoolcache/jfrog/[RELEASE]/x64:/opt/hostedtoolcache/jf/[RELEASE]/x64:/snap/bin:/home/runner/.local/bin:/opt/pipx_bin:/home/runner/.cargo/bin:/home/runner/.config/composer/vendor/bin:/usr/local/.ghcup/bin:/home/runner/.dotnet/tools:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/games:/usr/local/games:/snap/bin",
      "buildInfo.env.PIPX_HOME" : "/opt/pipx",
      "buildInfo.env.HOMEBREW_NO_AUTO_UPDATE" : "1",
      "buildInfo.env.SGX_AESM_ADDR" : "1",
      "buildInfo.env.STATS_D_D" : "true",
      "buildInfo.env.GITHUB_REF_PROTECTED" : "false",
      "buildInfo.env.ANDROID_NDK_LATEST_HOME" : "/usr/local/lib/android/sdk/ndk/26.3.11579264",
      "buildInfo.env.HOMEBREW_CLEANUP_PERIODIC_FULL_DAYS" : "3650",
      "buildInfo.env.EDGEWEBDRIVER" : "/usr/local/share/edge_driver",
      "buildInfo.env.DOTNET_MULTILEVEL_LOOKUP" : "0",
      "buildInfo.env.PIPX_BIN_DIR" : "/opt/pipx_bin",
      "buildInfo.env.GITHUB_ACTIONS" : "true",
      "buildInfo.env.STATS_EXT" : "true",
      "buildInfo.env.DOTNET_NOLOGO" : "1",
      "buildInfo.env.GITHUB_RETENTION_DAYS" : "90",
      "buildInfo.env.JAVA_HOME_8_X64" : "/usr/lib/jvm/temurin-8-jdk-amd64",
      "buildInfo.env.GITHUB_EVENT_NAME" : "push",
      "buildInfo.env.USER" : "runner",
      "buildInfo.env.DOTNET_SKIP_FIRST_TIME_EXPERIENCE" : "1",
      "buildInfo.env.RUNNER_TRACKING_ID" : "github_7cfa2548-71fe-4324-a94c-09d8208c1950",
      "buildInfo.env.XDG_CONFIG_HOME" : "/home/runner/.config",
      "buildInfo.env.JFROG_CLI_BUILD_NUMBER" : "10",
      "buildInfo.env.LEIN_JAR" : "/usr/local/lib/lein/self-installs/leiningen-2.11.2-standalone.jar",
      "buildInfo.env.RUNNER_TEMP" : "/home/runner/work/_temp",
      "buildInfo.env.SELENIUM_JAR_PATH" : "/usr/share/java/selenium-server.jar",
      "buildInfo.env._" : "/opt/hostedtoolcache/jf/[RELEASE]/x64/jf",
      "buildInfo.env.RUNNER_WORKSPACE" : "/home/runner/work/project-examples",
      "buildInfo.env.GITHUB_HEAD_REF" : "",
      "buildInfo.env.RUNNER_USER" : "runner",
      "buildInfo.env.GECKOWEBDRIVER" : "/usr/local/share/gecko_driver",
      "buildInfo.env.HOME" : "/home/runner",
      "buildInfo.env.POWERSHELL_DISTRIBUTION_CHANNEL" : "GitHub-Actions-ubuntu22",
      "buildInfo.env.STATS_UE" : "true",
      "buildInfo.env.STATS_V3PS" : "true",
      "buildInfo.env.GITHUB_GRAPHQL_URL" : "https://api.github.com/graphql",
      "buildInfo.env.STATS_EXTP" : "https://provjobdsettingscdn.blob.core.windows.net/settings/provjobdsettings-0.5.181+6/provjobd.data"
    },
    "version" : "1.0.1",
    "name" : "Master-Build",
    "number" : "10",
    "buildAgent" : {
      "name" : "GENERIC",
      "version" : "2.59.1"
    },
    "agent" : {
      "name" : "setup-jfrog-cli-github-action",
      "version" : "4.1.3"
    },
    "started" : "2024-07-22T16:25:15.711+0000",
    "durationMillis" : 0,
    "artifactoryPrincipal" : "robin",
    "url" : "https://github.com/RobiNino/project-examples/actions/runs/10044646858",
    "modules" : [ {
      "properties" : { },
      "type" : "generic",
      "id" : "jfrog-python-example",
      "artifacts" : [ {
        "type" : "whl",
        "sha1" : "79357de49c6ee791779c3941b3682fe97abfdd14",
        "sha256" : "dbda1c1922aa21663ec9e2ee957ef9aa0ad8ebcf92d050311a3ea554aa54de7d",
        "md5" : "904a180f1a6e29bb5c459dd9dbf659ef",
        "name" : "jfrog_python_example-1.0-py3-none-any.whl",
        "path" : "dist/jfrog_python_example-1.0-py3-none-any.whl"
      }, {
        "type" : "gz",
        "sha1" : "40669742b9253805bdb4aa1c9bdb181b818e3b64",
        "sha256" : "dad4825a36d67cbcdf11779ae31d661993d83f799f0c7f4d8ded8c94b4a879e7",
        "md5" : "311d3b3d22846534017106f06ade83e5",
        "name" : "jfrog_python_example-1.0.tar.gz",
        "path" : "dist/jfrog_python_example-1.0.tar.gz"
      } ],
      "dependencies" : [ {
        "type" : "whl",
        "sha1" : "f4d480f20f19d5f43e3a2c1045be5b20feec9115",
        "sha256" : "d2b04aac4d386b172d5b9692e2d2da8de7bfb6c387fa4f801fbf6fb2e6ba4673",
        "md5" : "7bc9f812a53efeda8d0927cdd7c0353b",
        "id" : "nltk:3.8.1",
        "requestedBy" : [ [ "jfrog-python-example" ] ]
      }, {
        "type" : "whl",
        "sha1" : "98258e6c72b3cb47b9ad191fbd236c87a9c26d30",
        "sha256" : "fd5c9109f976fa86bcadba8f91e47f5e9293bd034474752e92a520f81c93dda5",
        "md5" : "10023198f7c40bb0856583a9ad0f10bf",
        "id" : "click:8.1.7",
        "requestedBy" : [ [ "nltk:3.8.1", "jfrog-python-example" ] ]
      }, {
        "type" : "whl",
        "sha1" : "d066d29afdc61b0ba89ff9a58803fce84d47682b",
        "sha256" : "ae74fb96c20a0277a1d615f1e4d73c8414f5a98db8b799a7931d1582f3390c28",
        "md5" : "37a41134cc8a13400234746942d5d180",
        "id" : "joblib:1.4.2",
        "requestedBy" : [ [ "nltk:3.8.1", "jfrog-python-example" ] ]
      }, {
        "type" : "whl",
        "sha1" : "9dababf095ce71d64daed9b0a93588d73a7f57c1",
        "sha256" : "06d478d5674cbc267e7496a410ee875abd68e4340feff4490bcb7afb88060ae6",
        "md5" : "9f88e92dcec0663cf61eef0a83b35cd1",
        "id" : "regex:2024.5.15",
        "requestedBy" : [ [ "nltk:3.8.1", "jfrog-python-example" ] ]
      }, {
        "type" : "whl",
        "sha1" : "5e33e6df26863789791d54fb189a2de885e86687",
        "sha256" : "3e507ff1e74373c4d3038195fdd2af30d297b4f0950eeda6f515ae3d84a1770f",
        "md5" : "08566bd0c2f7b183d4065fe107120230",
        "id" : "tqdm:4.66.4",
        "requestedBy" : [ [ "nltk:3.8.1", "jfrog-python-example" ] ]
      }, {
        "type" : "whl",
        "sha1" : "f4d480f20f19d5f43e3a2c1045be5b20feec9115",
        "sha256" : "d2b04aac4d386b172d5b9692e2d2da8de7bfb6c387fa4f801fbf6fb2e6ba4673",
        "md5" : "7bc9f812a53efeda8d0927cdd7c0353b",
        "id" : "pyyaml:6.0.1",
        "requestedBy" : [ [ "jfrog-python-example" ] ]
      } ]
    }, {
      "properties" : { },
      "type" : "npm",
      "id" : "npm-example:0.0.3",
      "artifacts" : [ {
        "type" : "tgz",
        "sha1" : "2e90d6e34147dbbb8ce5895748ef8d72b75c7c36",
        "sha256" : "975fcea98e1c33d5bd94361a6491ec31d68d911d31210c8617b17f8f32b7d7ef",
        "md5" : "74f6cbfb2c2b02f1c21bd25e45505a40",
        "name" : "npm-example-0.0.3.tgz",
        "path" : "npm-example/-/npm-example-0.0.3.tgz"
      } ],
      "dependencies" : [ {
        "sha1" : "590c61156b0ae2f4f0255732a158b266bc56b21d",
        "sha256" : "5148e8eb7e222b2a09127618bbdb5033daf6262cfc735d3101ea98620128b99c",
        "md5" : "28731200888ff19ce1053590822317b8",
        "id" : "ee-first:1.1.1",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "on-finished:2.3.0", "send:0.16.2", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "5d128515df134ff327e90a4c93f4e077a536341f",
        "sha256" : "34ae48c66698f1f81e2a2e6e322f34e8a88b0986a3fa7b74bb5ea14c0edb1c98",
        "md5" : "cb6cb63ab5843aee3af94d27c60ea476",
        "id" : "debug:2.6.9",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "send:0.16.2", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "bb73d446da2796106efcc1b601a253d6c46bd087",
        "sha256" : "81e0aef6a3eec7052cb17e440353f8428cda89b0300988b43095fc196df48c2a",
        "md5" : "0bcdfe9b17d996d363a537c8fbb4040f",
        "id" : "statuses:1.4.0",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "http-errors:1.6.3", "send:0.16.2", "npm-example:0.0.3" ], [ "send:0.16.2", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "3d8cadd90d976569fa835ab1f8e4b23a105605a7",
        "sha256" : "25e676f9f00a8dddc7214f46258b71c0188a0ee004903f05109ecfce9a5a844c",
        "md5" : "2a40d6543d99d22ed619187b3669a099",
        "id" : "fresh:0.5.2",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "send:0.16.2", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "8b55680bb4be283a0b5bf4ea2e38580be1d9320d",
        "sha256" : "fe12a950f0f74f877e5ad2eb64c324075fcd9142eb53d8c3e4e699419298fa14",
        "md5" : "3ecddf4de4052831d3e99f7750bd8b95",
        "id" : "http-errors:1.6.3",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "send:0.16.2", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "20f1336481b083cd75337992a16971aa2d906947",
        "sha256" : "a9640d8669cd8de27158f39364a8ef98296b15e4eca861a9214f81e98696616b",
        "md5" : "745329d06dcc1e7d0c3bb19c98db92ff",
        "id" : "on-finished:2.3.0",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "send:0.16.2", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "6ecca1e0f8c156d141597559848df64730a6bbc1",
        "sha256" : "efefc8d4e996fa73fa66c28fb2742485ca751fc0cb1d112b5c3396e526e3690a",
        "md5" : "c78f9fbf3091196bc1125b5dd224b0fc",
        "id" : "send:0.16.2",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "0258eae4d3d0c0974de1c169188ef0051d1d1988",
        "sha256" : "a101155c3cbdfb1e4f98f2f83c8b5e392db6accfa606df0eba8b87a5762b0366",
        "md5" : "0e644d0c31d5f4c2eeeaef9566add5e9",
        "id" : "escape-html:1.0.3",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "send:0.16.2", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "9bcd52e14c097763e749b274c4346ed2e560b5a9",
        "sha256" : "83e26be6b5821152c78a0b247c8290c2cb5e0a0a7f8f673ef238487ae12bc41c",
        "md5" : "b18f932b694413126a04262fd6e148c8",
        "id" : "depd:1.1.2",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "send:0.16.2", "npm-example:0.0.3" ], [ "http-errors:1.6.3", "send:0.16.2", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "ad3ff4c86ec2d029322f5a02c3a9a606c95b3f59",
        "sha256" : "cb46598dbb11155157acb5eeeedf93dd9c4522783a9c92c4240a0ad259dc4e8d",
        "md5" : "150b36de9deff219af051490b05dc211",
        "id" : "encodeurl:1.0.2",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "send:0.16.2", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "633c2c83e3da42a502f52466022480f4208261de",
        "sha256" : "7f5f58e9b54e87e264786e7e84d9e078aaf68c1003de9fa68945101e02356cdf",
        "md5" : "5fab4bb68d920d26a1029377bb99fa46",
        "id" : "inherits:2.0.3",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "http-errors:1.6.3", "send:0.16.2", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "d0bd85536887b6fe7c0d818cb962d9d91c54e656",
        "sha256" : "e9ca9d42c3febb8e239da76d50455584afc481894b98fe7ff99950e97da4ed64",
        "md5" : "9f357801ad028600d9405c912e329bee",
        "id" : "setprototypeof:1.1.0",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "http-errors:1.6.3", "send:0.16.2", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "3cf37023d199e1c24d1a55b84800c2f3e6468031",
        "sha256" : "905266e2662590b009e38dc096d4e4e103aafda9cb67e658ca985b2d0ad2f926",
        "md5" : "03ee4757f00d0b3ddd2e31f1abde6bcd",
        "id" : "range-parser:1.2.1",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "send:0.16.2", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "e83444eceb9fedd4a1da56d671ae2446a01a6e1e",
        "sha256" : "c4929fdeaf8ec4c08ffbe88316b4db5e28080475f7cb90a494882d5b01bf3d1a",
        "md5" : "fb66c5f7f9a0a589a577f9d2995a8732",
        "id" : "debug:4.3.5",
        "scopes" : [ "dev" ],
        "requestedBy" : [ [ "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "5608aeadfc00be6c2901df5f9861788de0d597c8",
        "sha256" : "362152ab8864181fc3359a3c440eec58ce3e18f773b0dde4d88a84fe13d73ecb",
        "md5" : "9615634070dd7751f127b2a0fb362484",
        "id" : "ms:2.0.0",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "debug:2.6.9", "send:0.16.2", "npm-example:0.0.3" ], [ "send:0.16.2", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "41ae2eeb65efa62268aebfea83ac7d79299b0887",
        "sha256" : "f6a96c78a973d2ab660c9efeee6aa74a399cd9e770625ba1ed95e1aca9fd0faf",
        "md5" : "1ccf0041293792c96d81db7f15c3561d",
        "id" : "etag:1.8.1",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "send:0.16.2", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "121f9ebc49e3766f311a76e1fa1c8003c4b03aa6",
        "sha256" : "bd7568ce30be73f166e7cfbb65a6cabf641a82fd67b2cedac8c1f22416e5d130",
        "md5" : "d495c480c73869d4e203951fde48ebeb",
        "id" : "mime:1.4.1",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "send:0.16.2", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "d09d1f357b443f493382a8eb3ccd183872ae6009",
        "sha256" : "1157a6e30d3ffe1b9fcaf3a39caf159f8dc981199a3380c78ddd89f73bcefb48",
        "md5" : "5a8310f20fd4b97c7f8eeaf65f896a7a",
        "id" : "ms:2.1.2",
        "scopes" : [ "dev" ],
        "requestedBy" : [ [ "debug:4.3.5", "npm-example:0.0.3" ] ]
      }, {
        "sha1" : "978857442c44749e4206613e37946205826abd80",
        "sha256" : "5b579622f4650fdd0effe150fca231e5a8b4cbc464430424a1e730c785423e56",
        "md5" : "8739ecad471c4e32cdabb7e49a2a7810",
        "id" : "destroy:1.0.4",
        "scopes" : [ "prod" ],
        "requestedBy" : [ [ "send:0.16.2", "npm-example:0.0.3" ] ]
      } ]
    }, {
      "properties" : { },
      "type" : "go",
      "id" : "github.com/you/hello",
      "artifacts" : [ {
        "type" : "zip",
        "sha1" : "caed4a0891e1c8b8b9e788b97c7d3642da85fce4",
        "sha256" : "69a49d9c39d84051f18a634aecf56da2540ad8a07ac51c1151ec3c460519e14a",
        "md5" : "b33dce5aed1b85f53c2f3bbfb9d620b3",
        "name" : "v1.0.0.zip"
      }, {
        "type" : "info",
        "sha1" : "b97f4610ff55656e0c6f0380e18a36a4aa5471ba",
        "sha256" : "d431c7176018d39700f5bb22ca44c6c1e9b43071f7af90b1286540961316c2d8",
        "md5" : "e89b3d76827f9693761ff6d46002b02d",
        "name" : "v1.0.0.info"
      }, {
        "type" : "mod",
        "sha1" : "4b95125be2c6b555dc374892c28e7cd7b59bce0e",
        "sha256" : "2390edc8f245190501cc9ab1d52c6eac2fff58f820c06d5e300db760db3a8a80",
        "md5" : "9183f677b096f061d26a0ae9f95d406f",
        "name" : "v1.0.0.mod"
      } ],
      "dependencies" : [ {
        "type" : "zip",
        "sha1" : "1eaf56fb5889f2828975a0e1fca81bec68ae41b1",
        "sha256" : "da202b0da803ab2661ab98a680bba4f64123a326e540c25582b6cdbb9dc114aa",
        "md5" : "505bcacd8b389903d3972c1e16b6afc3",
        "id" : "rsc.io/sampler:v1.3.0",
        "requestedBy" : [ [ "rsc.io/quote:v1.5.2", "github.com/you/hello" ], [ "github.com/you/hello" ] ]
      }, {
        "type" : "zip",
        "sha1" : "be754980e1331d561d5777a577c3986044a6fe9c",
        "sha256" : "119de6cd7e06055a33f51c65b0acdf74231a02c50edbd96bfa9c5ed0a1b0050d",
        "md5" : "fa81ea7c3c3536fa4a324fc98322dac8",
        "id" : "golang.org/x/text:v0.0.0-20170915032832-14c0d48ead0c",
        "requestedBy" : [ [ "github.com/you/hello" ], [ "rsc.io/sampler:v1.3.0", "rsc.io/quote:v1.5.2", "github.com/you/hello" ], [ "rsc.io/sampler:v1.3.0", "github.com/you/hello" ] ]
      }, {
        "type" : "zip",
        "sha1" : "2fa7fd59a8e5d5f41af275cf3ed0b64a83d0734a",
        "sha256" : "643fcf8ef4e4cbb8f910622c42df3f9a81f3efe8b158a05825a81622c121ca0a",
        "md5" : "61a79aa27670b571cc17248fe167a14b",
        "id" : "rsc.io/quote:v1.5.2",
        "requestedBy" : [ [ "github.com/you/hello" ] ]
      } ]
    }, {
      "properties" : { },
      "type" : "generic",
      "id" : "generic-module",
      "artifacts" : [ {
        "type" : "sh",
        "sha1" : "4980a89303820f4dea6d55fea39bc212649ae03b",
        "sha256" : "654effd2207ce9bf042f7ace3a7413f7d281097b0daa6bd7e881f3759247a232",
        "md5" : "9ed9de9545a604aa9312aa5d40b910d9",
        "name" : "deploy-file.sh",
        "path" : "deploy-file.sh"
      } ]
    }, {
      "properties" : {
        "docker.image.tag" : "ecosysjfrog.jfrog.io/docker-local/multiarch-image:1"
      },
      "type" : "docker",
      "id" : "multiarch-image:1",
      "artifacts" : [ {
        "type" : "json",
        "sha1" : "33b57395a4070d594d6ebd70282d74ff900e0d6f",
        "sha256" : "f595a0dcf05f0c6cb6c66d2fc39e4e3f704ffa2d5848be98f82cf5f6c594189c",
        "md5" : "d301f1503152787bb3c76da5656d28ce",
        "name" : "list.manifest.json",
        "path" : "multiarch-image/1/list.manifest.json"
      } ]
    }, {
      "type" : "docker",
      "id" : "linux/amd64/multiarch-image:1",
      "artifacts" : [ {
        "sha1" : "32c1416f8430fbbabd82cb014c5e09c5fe702404",
        "sha256" : "aee9d258e62f0666e3286acca21be37d2e39f69f8dde74454b9f3cd8ef437e4e",
        "md5" : "f568bfb1c9576a1f06235ebe0389d2d8",
        "name" : "sha256__aee9d258e62f0666e3286acca21be37d2e39f69f8dde74454b9f3cd8ef437e4e",
        "path" : "multiarch-image/sha256:552ccb2628970ef526f13151a0269258589fc8b5701519a9c255c4dd224b9a21/sha256__aee9d258e62f0666e3286acca21be37d2e39f69f8dde74454b9f3cd8ef437e4e"
      }, {
        "sha1" : "18a91e823f0e8a11f39bfdef97c293819ba7e6b1",
        "sha256" : "59bf1c3509f33515622619af21ed55bbe26d24913cedbca106468a5fb37a50c3",
        "md5" : "1d55e7be5a77c4a908ad11bc33ebea1c",
        "name" : "sha256__59bf1c3509f33515622619af21ed55bbe26d24913cedbca106468a5fb37a50c3",
        "path" : "multiarch-image/sha256:552ccb2628970ef526f13151a0269258589fc8b5701519a9c255c4dd224b9a21/sha256__59bf1c3509f33515622619af21ed55bbe26d24913cedbca106468a5fb37a50c3"
      }, {
        "type" : "json",
        "sha1" : "e72625618204895d7e309299b0d068e85b6d730a",
        "sha256" : "552ccb2628970ef526f13151a0269258589fc8b5701519a9c255c4dd224b9a21",
        "md5" : "919495783ab6378cd5d68df531346c20",
        "name" : "manifest.json",
        "path" : "multiarch-image/sha256:552ccb2628970ef526f13151a0269258589fc8b5701519a9c255c4dd224b9a21/manifest.json"
      } ]
    }, {
      "type" : "docker",
      "id" : "linux/arm64/multiarch-image:1",
      "artifacts" : [ {
        "sha1" : "82b6d4ae1f673c609469a0a84170390ecdff5a38",
        "sha256" : "1f17f9d95f85ba55773db30ac8e6fae894831be87f5c28f2b58d17f04ef65e93",
        "md5" : "d178dd8c1e1fded51ade114136ebdaf2",
        "name" : "sha256__1f17f9d95f85ba55773db30ac8e6fae894831be87f5c28f2b58d17f04ef65e93",
        "path" : "multiarch-image/sha256:bee6dc0408dfd20c01e12e644d8bc1d60ff100a8c180d6c7e85d374c13ae4f92/sha256__1f17f9d95f85ba55773db30ac8e6fae894831be87f5c28f2b58d17f04ef65e93"
      }, {
        "sha1" : "d655adf5cc1106b7ee80af5d1af3f31dd8066e93",
        "sha256" : "9b3977197b4f2147bdd31e1271f811319dcd5c2fc595f14e81f5351ab6275b99",
        "md5" : "a1912764fd57f5588f48fc1bfb12e318",
        "name" : "sha256__9b3977197b4f2147bdd31e1271f811319dcd5c2fc595f14e81f5351ab6275b99",
        "path" : "multiarch-image/sha256:bee6dc0408dfd20c01e12e644d8bc1d60ff100a8c180d6c7e85d374c13ae4f92/sha256__9b3977197b4f2147bdd31e1271f811319dcd5c2fc595f14e81f5351ab6275b99"
      }, {
        "type" : "json",
        "sha1" : "942a85b5e2e02baa016a99a28bbe12c8b2892c33",
        "sha256" : "bee6dc0408dfd20c01e12e644d8bc1d60ff100a8c180d6c7e85d374c13ae4f92",
        "md5" : "015505d17214f12aca77538ce9ff2697",
        "name" : "manifest.json",
        "path" : "multiarch-image/sha256:bee6dc0408dfd20c01e12e644d8bc1d60ff100a8c180d6c7e85d374c13ae4f92/manifest.json"
      } ]
    }, {
      "type" : "docker",
      "id" : "linux/arm/multiarch-image:1",
      "artifacts" : [ {
        "sha1" : "63d3ac90f9cd322b76543d7bf96eeb92417faf41",
        "sha256" : "33b5b5485e88e63d3630e5dcb008f98f102b0f980a9daa31bd976efdec7a8e4c",
        "md5" : "99bbb1e1035aea4d9150e4348f24e107",
        "name" : "sha256__33b5b5485e88e63d3630e5dcb008f98f102b0f980a9daa31bd976efdec7a8e4c",
        "path" : "multiarch-image/sha256:686085b9972e0f7a432b934574e3dca27b4fa0a3d10d0ae7099010160db6d338/sha256__33b5b5485e88e63d3630e5dcb008f98f102b0f980a9daa31bd976efdec7a8e4c"
      }, {
        "sha1" : "9dceac352f990a3149ff97ab605c3c8833409abf",
        "sha256" : "5480d2ca1740c20ce17652e01ed2265cdc914458acd41256a2b1ccff28f2762c",
        "md5" : "d6a694604c7e58b2c788dec5656a1add",
        "name" : "sha256__5480d2ca1740c20ce17652e01ed2265cdc914458acd41256a2b1ccff28f2762c",
        "path" : "multiarch-image/sha256:686085b9972e0f7a432b934574e3dca27b4fa0a3d10d0ae7099010160db6d338/sha256__5480d2ca1740c20ce17652e01ed2265cdc914458acd41256a2b1ccff28f2762c"
      }, {
        "type" : "json",
        "sha1" : "d9d3575794ffc46ddb2e5a5df9b795197be47b2d",
        "sha256" : "686085b9972e0f7a432b934574e3dca27b4fa0a3d10d0ae7099010160db6d338",
        "md5" : "e0d1d2c6420cdff60c6704f5726b569f",
        "name" : "manifest.json",
        "path" : "multiarch-image/sha256:686085b9972e0f7a432b934574e3dca27b4fa0a3d10d0ae7099010160db6d338/manifest.json"
      } ]
    }, {
      "type" : "docker",
      "id" : "unknown/unknown/multiarch-image:1",
      "artifacts" : [ {
        "sha1" : "e94af9c1da6f58b60a4672e1380ec5b10e6d5df0",
        "sha256" : "bd6ab18cd1170a422dcb3e6636f3c51661a4a332bdf5a24a79c02daadc220287",
        "md5" : "26acf474b3439da9d335d77ec2fc5478",
        "name" : "sha256__bd6ab18cd1170a422dcb3e6636f3c51661a4a332bdf5a24a79c02daadc220287",
        "path" : "multiarch-image/sha256:a6ffa00855f1e43618b3286e78ba9f5c29aab624865dc956f29b02fb02a8bdcb/sha256__bd6ab18cd1170a422dcb3e6636f3c51661a4a332bdf5a24a79c02daadc220287"
      }, {
        "sha1" : "3ef21cd0f04cbc09296648978d3a66350cd0dd8e",
        "sha256" : "0dd201d2a61c0f366bae964bfab163b733151ba4be38fa4ed9b2e6f525b9dcf1",
        "md5" : "84409ecb481e7158b9f0edb2f29c6636",
        "name" : "sha256__0dd201d2a61c0f366bae964bfab163b733151ba4be38fa4ed9b2e6f525b9dcf1",
        "path" : "multiarch-image/sha256:a6ffa00855f1e43618b3286e78ba9f5c29aab624865dc956f29b02fb02a8bdcb/sha256__0dd201d2a61c0f366bae964bfab163b733151ba4be38fa4ed9b2e6f525b9dcf1"
      }, {
        "type" : "json",
        "sha1" : "38db5193c3f182c5e078591c5076bcb2274825d0",
        "sha256" : "a6ffa00855f1e43618b3286e78ba9f5c29aab624865dc956f29b02fb02a8bdcb",
        "md5" : "2de74de2e9237e63923cc86bdee9ff4f",
        "name" : "manifest.json",
        "path" : "multiarch-image/sha256:a6ffa00855f1e43618b3286e78ba9f5c29aab624865dc956f29b02fb02a8bdcb/manifest.json"
      }, {
        "sha1" : "1961cc61800b331b581e21a2390361425a2eed87",
        "sha256" : "a09324a533c701f65f52aa3def5bb07c5161c46b9182020bb65a2a360d263403",
        "md5" : "ec82ebf5d86afa591d5bca19a9a46f64",
        "name" : "sha256__a09324a533c701f65f52aa3def5bb07c5161c46b9182020bb65a2a360d263403",
        "path" : "multiarch-image/sha256:8ea9fa81071bb6151c99880be241828b08e4bed1cde1a430b0ac6716b7e23162/sha256__a09324a533c701f65f52aa3def5bb07c5161c46b9182020bb65a2a360d263403"
      }, {
        "sha1" : "0b565dbba361a5453267fdadedb43ac58109884e",
        "sha256" : "652ca9deaf58ffe3e92d502009ff04b028052e2cfb306381ba98d9b4a3a0ca4a",
        "md5" : "64e6d0ac97acc8cb7233a124b132187a",
        "name" : "sha256__652ca9deaf58ffe3e92d502009ff04b028052e2cfb306381ba98d9b4a3a0ca4a",
        "path" : "multiarch-image/sha256:8ea9fa81071bb6151c99880be241828b08e4bed1cde1a430b0ac6716b7e23162/sha256__652ca9deaf58ffe3e92d502009ff04b028052e2cfb306381ba98d9b4a3a0ca4a"
      }, {
        "type" : "json",
        "sha1" : "b24fce781ac161bb3dd3e708b99cf2b6c9061290",
        "sha256" : "8ea9fa81071bb6151c99880be241828b08e4bed1cde1a430b0ac6716b7e23162",
        "md5" : "947f79e53777394b733e2a0b42de6231",
        "name" : "manifest.json",
        "path" : "multiarch-image/sha256:8ea9fa81071bb6151c99880be241828b08e4bed1cde1a430b0ac6716b7e23162/manifest.json"
      }, {
        "sha1" : "5ba88567cda3ac796619422bba34935490ae0149",
        "sha256" : "855507e5199a22a343e37e1fc515683f072c036d1987750f72056d3e615f5e88",
        "md5" : "510d92e833e3f7610e3dbd3a73e9c2f9",
        "name" : "sha256__855507e5199a22a343e37e1fc515683f072c036d1987750f72056d3e615f5e88",
        "path" : "multiarch-image/sha256:e21a93a7ade7d310654990ad1ddd116b14e66f22b59b34d14e6bbf6116943169/sha256__855507e5199a22a343e37e1fc515683f072c036d1987750f72056d3e615f5e88"
      }, {
        "sha1" : "6cdb8aeeabbbd4ad91d1afb778dd1c0d48a2d4c7",
        "sha256" : "b9b648976e25e75f5a556a8b8e677cc6ea1511d016f2276226dec3ce076fba1b",
        "md5" : "5e85d47efa2a35a4aa13bdd50e027461",
        "name" : "sha256__b9b648976e25e75f5a556a8b8e677cc6ea1511d016f2276226dec3ce076fba1b",
        "path" : "multiarch-image/sha256:e21a93a7ade7d310654990ad1ddd116b14e66f22b59b34d14e6bbf6116943169/sha256__b9b648976e25e75f5a556a8b8e677cc6ea1511d016f2276226dec3ce076fba1b"
      }, {
        "type" : "json",
        "sha1" : "995a7a068cccb8a9103271198443842b1a910f9b",
        "sha256" : "e21a93a7ade7d310654990ad1ddd116b14e66f22b59b34d14e6bbf6116943169",
        "md5" : "7a120a6e728d39dbd9c65ed05910d40c",
        "name" : "manifest.json",
        "path" : "multiarch-image/sha256:e21a93a7ade7d310654990ad1ddd116b14e66f22b59b34d14e6bbf6116943169/manifest.json"
      } ]
    }, {
      "properties" : {
        "maven.compiler.target" : "1.8",
        "maven.compiler.source" : "1.8",
        "project.build.sourceEncoding" : "UTF-8",
        "daversion" : "3.7-SNAPSHOT"
      },
      "type" : "maven",
      "id" : "org.jfrog.test:multi2:3.7-SNAPSHOT",
      "artifacts" : [ {
        "type" : "jar",
        "sha1" : "7e8053f4c1ceb6bc0b0df08f1e83a83835a32363",
        "sha256" : "1858e133102ada2b402c7c5a3157b9d729982907d7daeb7e3590e6b8ff7a46c2",
        "md5" : "be86c200a462fa21c035f7bc72cf56ce",
        "name" : "multi2-3.7-SNAPSHOT.jar",
        "path" : "org/jfrog/test/multi2/3.7-SNAPSHOT/multi2-3.7-SNAPSHOT.jar"
      }, {
        "type" : "pom",
        "sha1" : "ce4de242f1f9ee88339887613612f3a0551267fd",
        "sha256" : "bf6a6f994efc0c10cf3ab30a22bbbfd103c7b0f83053d9fe7cd346ef49ab1e26",
        "md5" : "35f626d44fd38f8c221aa3d78af64aa9",
        "name" : "multi2-3.7-SNAPSHOT.pom",
        "path" : "org/jfrog/test/multi2/3.7-SNAPSHOT/multi2-3.7-SNAPSHOT.pom"
      } ],
      "dependencies" : [ {
        "type" : "jar",
        "sha1" : "99129f16442844f6a4a11ae22fbbee40b14d774f",
        "sha256" : "b58e459509e190bed737f3592bc1950485322846cf10e78ded1d065153012d70",
        "md5" : "1f40fb782a4f2cf78f161d32670f7a3a",
        "id" : "junit:junit:3.8.1",
        "scopes" : [ "test" ],
        "requestedBy" : [ [ "org.jfrog.test:multi2:3.7-SNAPSHOT" ] ]
      } ]
    }, {
      "properties" : {
        "maven.compiler.target" : "1.8",
        "maven.compiler.source" : "1.8",
        "project.build.sourceEncoding" : "UTF-8"
      },
      "type" : "maven",
      "id" : "org.jfrog.test:multi1:3.7-SNAPSHOT",
      "artifacts" : [ {
        "type" : "test-jar",
        "sha1" : "a9c99383d0b121c03677165f398ce357a2dfb44d",
        "sha256" : "79f09af967074bbb70b76962568c6faa67e339194f60c530decdc00ea59424dc",
        "md5" : "390f45f34a305861aa569cb749f7be79",
        "name" : "multi1-3.7-SNAPSHOT-tests.jar",
        "path" : "org/jfrog/test/multi1/3.7-SNAPSHOT/multi1-3.7-SNAPSHOT-tests.jar"
      }, {
        "type" : "java-source-jar",
        "sha1" : "7d72bf17febbc1db5900897cc01e269c901341cc",
        "sha256" : "b03786dcaa31417ff7b685e932ac5ed06c3409384b359d3bd9e8b3f67e7c8fd3",
        "md5" : "0c91601f3ac949949607880cf3095089",
        "name" : "multi1-3.7-SNAPSHOT-sources.jar",
        "path" : "org/jfrog/test/multi1/3.7-SNAPSHOT/multi1-3.7-SNAPSHOT-sources.jar"
      }, {
        "type" : "jar",
        "sha1" : "7ba2ecc62a60e54ab5f2e9397d2924660fba51de",
        "sha256" : "0a68505dc048147d84b988bbb6c537cebee7ee0bde756fcae42f3d48f653246f",
        "md5" : "af2ef05d6eb88380b42c00897bdc0cb8",
        "name" : "multi1-3.7-SNAPSHOT.jar",
        "path" : "org/jfrog/test/multi1/3.7-SNAPSHOT/multi1-3.7-SNAPSHOT.jar"
      }, {
        "type" : "pom",
        "sha1" : "bf793dfd54eb7d21332cc0f546572ce1a8c0abdb",
        "sha256" : "c07a3407dddb58a2daa42b2b117814d5545bcfece45c0cfffb21216eea072722",
        "md5" : "40cceb21b38d85a34972835e2ee8e0d6",
        "name" : "multi1-3.7-SNAPSHOT.pom",
        "path" : "org/jfrog/test/multi1/3.7-SNAPSHOT/multi1-3.7-SNAPSHOT.pom"
      } ],
      "dependencies" : [ {
        "type" : "jar",
        "sha1" : "449ea46b27426eb846611a90b2fb8b4dcf271191",
        "sha256" : "d33246bb33527685d04f23536ebf91b06ad7fa8b371fcbeb12f01523eb610104",
        "md5" : "25c0752852205167af8f31a1eb019975",
        "id" : "org.springframework:spring-beans:2.5.6",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.springframework:spring-aop:2.5.6", "org.jfrog.test:multi1:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "5043bfebc3db072ed80fbd362e7caf00e885d8ae",
        "sha256" : "ce6f913cad1f0db3aad70186d65c5bc7ffcc9a99e3fe8e0b137312819f7c362f",
        "md5" : "ed448347fc0104034aa14c8189bf37de",
        "id" : "commons-logging:commons-logging:1.1.1",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.springframework:spring-aop:2.5.6", "org.jfrog.test:multi1:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "a8762d07e76cfde2395257a5da47ba7c1dbd3dce",
        "sha256" : "a7f713593007813bf07d19bd1df9f81c86c0719e9a0bb2ef1b98b78313fc940d",
        "md5" : "b6a50c8a15ece8753e37cbe5700bf84f",
        "id" : "commons-io:commons-io:1.4",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.jfrog.test:multi1:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "0235ba8b489512805ac13a8f9ea77a1ca5ebe3e8",
        "sha256" : "0addec670fedcd3f113c5c8091d783280d23f75e3acb841b61a9cdb079376a08",
        "md5" : "04177054e180d09e3998808efa0401c7",
        "id" : "aopalliance:aopalliance:1.0",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.springframework:spring-aop:2.5.6", "org.jfrog.test:multi1:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "63f943103f250ef1f3a4d5e94d145a0f961f5316",
        "sha256" : "545f4e7dc678ffb4cf8bd0fd40b4a4470a409a787c0ea7d0ad2f08d56112987b",
        "md5" : "b8a34113a3a1ce29c8c60d7141f5a704",
        "id" : "javax.servlet.jsp:jsp-api:2.1",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.jfrog.test:multi1:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "a05c4de7bf2e0579ac0f21e16f3737ec6fa0ff98",
        "sha256" : "78da962833d83a9df219d07b6c8c60115a0146a7314f8e44df3efdcf15792eaa",
        "md5" : "5d6576b5b572c6d644af2924da9a1952",
        "id" : "org.apache.commons:commons-email:1.1",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.jfrog.test:multi1:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jdk15-jar",
        "sha1" : "9b8614979f3d093683d8249f61a9b6a29bc61d3d",
        "sha256" : "13e43a36008719957314bc9bfa2380e7a5881ccd364003687275b782ca4c62a6",
        "md5" : "52537a8a5231ca74518aec08434df7eb",
        "id" : "org.testng:testng:5.9",
        "scopes" : [ "test" ],
        "requestedBy" : [ [ "org.jfrog.test:multi1:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "99129f16442844f6a4a11ae22fbbee40b14d774f",
        "sha256" : "b58e459509e190bed737f3592bc1950485322846cf10e78ded1d065153012d70",
        "md5" : "1f40fb782a4f2cf78f161d32670f7a3a",
        "id" : "junit:junit:3.8.1",
        "scopes" : [ "test" ],
        "requestedBy" : [ [ "org.jfrog.test:multi1:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "e6cb541461c2834bdea3eb920f1884d1eb508b50",
        "sha256" : "2881c79c9d6ef01c58e62beea13e9d1ac8b8baa16f2fc198ad6e6776defdcdd3",
        "md5" : "8ae38e87cd4f86059c0294a8fe3e0b18",
        "id" : "javax.activation:activation:1.1",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.apache.commons:commons-email:1.1", "org.jfrog.test:multi1:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "342d1eb41a2bc7b52fa2e54e9872463fc86e2650",
        "sha256" : "72582f8ba285601fa753ceeda73ff3cbd94c6e78f52ec611621eaa0186165452",
        "md5" : "2a666534a425add50d017d4aa06a6fca",
        "id" : "org.codehaus.plexus:plexus-utils:1.5.1",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.jfrog.test:multi1:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "7d04f648dd88a2173ee7ff7bcb337a080ee5bea1",
        "sha256" : "32dff5cc773ebf023e2fcd1e96313360ec92362a93f74e7370d7dfda75bbe004",
        "md5" : "036c65b02a789306fbadd3c330f1e055",
        "id" : "org.springframework:spring-aop:2.5.6",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.jfrog.test:multi1:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "1aa1579ae5ecd41920c4f355b0a9ef40b68315dd",
        "sha256" : "96868f82264ebd9b7d41f04d78cbe87ab75d68a7bbf8edfb82416aabe9b54b6c",
        "md5" : "2e64a3805d543bdb86e6e5eeca5529f8",
        "id" : "javax.mail:mail:1.4",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.apache.commons:commons-email:1.1", "org.jfrog.test:multi1:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "c450bc49099430e13d21548d1e3d1a564b7e35e9",
        "sha256" : "cf37656069488043c47f49a5520bb06d6879b63ef6044abb200c51a7ff2d6c49",
        "md5" : "378db2cc1fbdd9ed05dff2dc1023963e",
        "id" : "org.springframework:spring-core:2.5.6",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.springframework:spring-aop:2.5.6", "org.jfrog.test:multi1:3.7-SNAPSHOT" ] ]
      } ]
    }, {
      "properties" : {
        "maven.compiler.target" : "1.8",
        "maven.compiler.source" : "1.8",
        "project.build.sourceEncoding" : "UTF-8"
      },
      "type" : "maven",
      "id" : "org.jfrog.test:multi:3.7-SNAPSHOT",
      "artifacts" : [ {
        "type" : "pom",
        "sha1" : "8da7e77e032366e78d717bb1ce06fb24f6583cfd",
        "sha256" : "d45612a49776755c871331f61ed8cbd23f27fa82e621dc8d157d822e4c5afe32",
        "md5" : "431b421c7fb3e606a65bc7c52dabeb53",
        "name" : "multi-3.7-SNAPSHOT.pom",
        "path" : "org/jfrog/test/multi/3.7-SNAPSHOT/multi-3.7-SNAPSHOT.pom"
      } ]
    }, {
      "properties" : {
        "maven.compiler.target" : "1.8",
        "maven.compiler.source" : "1.8",
        "project.build.sourceEncoding" : "UTF-8"
      },
      "type" : "maven",
      "id" : "org.jfrog.test:multi3:3.7-SNAPSHOT",
      "artifacts" : [ {
        "type" : "war",
        "sha1" : "548dda955c4dab5e67e696c360d99c9264f96b48",
        "sha256" : "4a895b4ec980bccfaa187f7f1c544864a68b9268e83705a9d0482024c4e54661",
        "md5" : "a857d81db1ffeea89bfd4edcb4bb7206",
        "name" : "multi3-3.7-SNAPSHOT.war",
        "path" : "org/jfrog/test/multi3/3.7-SNAPSHOT/multi3-3.7-SNAPSHOT.war"
      }, {
        "type" : "pom",
        "sha1" : "b718937536949885972898f24e735a186150f999",
        "sha256" : "4b71df64dd66e89d1cfdd4b3d37518be6275437f91497ba90c6ced9742681fd8",
        "md5" : "a9528a0973b2fa198399f1c5fb3f2438",
        "name" : "multi3-3.7-SNAPSHOT.pom",
        "path" : "org/jfrog/test/multi3/3.7-SNAPSHOT/multi3-3.7-SNAPSHOT.pom"
      } ],
      "dependencies" : [ {
        "type" : "jar",
        "sha1" : "449ea46b27426eb846611a90b2fb8b4dcf271191",
        "sha256" : "d33246bb33527685d04f23536ebf91b06ad7fa8b371fcbeb12f01523eb610104",
        "md5" : "25c0752852205167af8f31a1eb019975",
        "id" : "org.springframework:spring-beans:2.5.6",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.springframework:spring-aop:2.5.6", "org.jfrog.test:multi1:3.7-SNAPSHOT", "org.jfrog.test:multi3:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "7ba2ecc62a60e54ab5f2e9397d2924660fba51de",
        "sha256" : "0a68505dc048147d84b988bbb6c537cebee7ee0bde756fcae42f3d48f653246f",
        "md5" : "af2ef05d6eb88380b42c00897bdc0cb8",
        "id" : "org.jfrog.test:multi1:3.7-SNAPSHOT",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.jfrog.test:multi3:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "5043bfebc3db072ed80fbd362e7caf00e885d8ae",
        "sha256" : "ce6f913cad1f0db3aad70186d65c5bc7ffcc9a99e3fe8e0b137312819f7c362f",
        "md5" : "ed448347fc0104034aa14c8189bf37de",
        "id" : "commons-logging:commons-logging:1.1.1",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.springframework:spring-aop:2.5.6", "org.jfrog.test:multi1:3.7-SNAPSHOT", "org.jfrog.test:multi3:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "a8762d07e76cfde2395257a5da47ba7c1dbd3dce",
        "sha256" : "a7f713593007813bf07d19bd1df9f81c86c0719e9a0bb2ef1b98b78313fc940d",
        "md5" : "b6a50c8a15ece8753e37cbe5700bf84f",
        "id" : "commons-io:commons-io:1.4",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.jfrog.test:multi1:3.7-SNAPSHOT", "org.jfrog.test:multi3:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "0235ba8b489512805ac13a8f9ea77a1ca5ebe3e8",
        "sha256" : "0addec670fedcd3f113c5c8091d783280d23f75e3acb841b61a9cdb079376a08",
        "md5" : "04177054e180d09e3998808efa0401c7",
        "id" : "aopalliance:aopalliance:1.0",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.springframework:spring-aop:2.5.6", "org.jfrog.test:multi1:3.7-SNAPSHOT", "org.jfrog.test:multi3:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "63f943103f250ef1f3a4d5e94d145a0f961f5316",
        "sha256" : "545f4e7dc678ffb4cf8bd0fd40b4a4470a409a787c0ea7d0ad2f08d56112987b",
        "md5" : "b8a34113a3a1ce29c8c60d7141f5a704",
        "id" : "javax.servlet.jsp:jsp-api:2.1",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.jfrog.test:multi1:3.7-SNAPSHOT", "org.jfrog.test:multi3:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "7e9978fdb754bce5fcd5161133e7734ecb683036",
        "sha256" : "b04b3b3ac295d497c87230eeb4f888327a5a15b9c3c1567db202a51d83ac9e41",
        "md5" : "7df83e09e41d742cc5fb20d16b80729c",
        "id" : "hsqldb:hsqldb:1.8.0.10",
        "scopes" : [ "runtime" ],
        "requestedBy" : [ [ "org.jfrog.test:multi3:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "a05c4de7bf2e0579ac0f21e16f3737ec6fa0ff98",
        "sha256" : "78da962833d83a9df219d07b6c8c60115a0146a7314f8e44df3efdcf15792eaa",
        "md5" : "5d6576b5b572c6d644af2924da9a1952",
        "id" : "org.apache.commons:commons-email:1.1",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.jfrog.test:multi1:3.7-SNAPSHOT", "org.jfrog.test:multi3:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "99129f16442844f6a4a11ae22fbbee40b14d774f",
        "sha256" : "b58e459509e190bed737f3592bc1950485322846cf10e78ded1d065153012d70",
        "md5" : "1f40fb782a4f2cf78f161d32670f7a3a",
        "id" : "junit:junit:3.8.1",
        "scopes" : [ "test" ],
        "requestedBy" : [ [ "org.jfrog.test:multi3:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "e6cb541461c2834bdea3eb920f1884d1eb508b50",
        "sha256" : "2881c79c9d6ef01c58e62beea13e9d1ac8b8baa16f2fc198ad6e6776defdcdd3",
        "md5" : "8ae38e87cd4f86059c0294a8fe3e0b18",
        "id" : "javax.activation:activation:1.1",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.apache.commons:commons-email:1.1", "org.jfrog.test:multi1:3.7-SNAPSHOT", "org.jfrog.test:multi3:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "342d1eb41a2bc7b52fa2e54e9872463fc86e2650",
        "sha256" : "72582f8ba285601fa753ceeda73ff3cbd94c6e78f52ec611621eaa0186165452",
        "md5" : "2a666534a425add50d017d4aa06a6fca",
        "id" : "org.codehaus.plexus:plexus-utils:1.5.1",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.jfrog.test:multi1:3.7-SNAPSHOT", "org.jfrog.test:multi3:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "7d04f648dd88a2173ee7ff7bcb337a080ee5bea1",
        "sha256" : "32dff5cc773ebf023e2fcd1e96313360ec92362a93f74e7370d7dfda75bbe004",
        "md5" : "036c65b02a789306fbadd3c330f1e055",
        "id" : "org.springframework:spring-aop:2.5.6",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.jfrog.test:multi1:3.7-SNAPSHOT", "org.jfrog.test:multi3:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "5959582d97d8b61f4d154ca9e495aafd16726e34",
        "sha256" : "c658ea360a70faeeadb66fb3c90a702e4142a0ab7768f9ae9828678e0d9ad4dc",
        "md5" : "69ca51af4e9a67a1027a7f95b52c3e8f",
        "id" : "javax.servlet:servlet-api:2.5",
        "scopes" : [ "provided" ],
        "requestedBy" : [ [ "org.jfrog.test:multi3:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "1aa1579ae5ecd41920c4f355b0a9ef40b68315dd",
        "sha256" : "96868f82264ebd9b7d41f04d78cbe87ab75d68a7bbf8edfb82416aabe9b54b6c",
        "md5" : "2e64a3805d543bdb86e6e5eeca5529f8",
        "id" : "javax.mail:mail:1.4",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.apache.commons:commons-email:1.1", "org.jfrog.test:multi1:3.7-SNAPSHOT", "org.jfrog.test:multi3:3.7-SNAPSHOT" ] ]
      }, {
        "type" : "jar",
        "sha1" : "c450bc49099430e13d21548d1e3d1a564b7e35e9",
        "sha256" : "cf37656069488043c47f49a5520bb06d6879b63ef6044abb200c51a7ff2d6c49",
        "md5" : "378db2cc1fbdd9ed05dff2dc1023963e",
        "id" : "org.springframework:spring-core:2.5.6",
        "scopes" : [ "compile" ],
        "requestedBy" : [ [ "org.springframework:spring-aop:2.5.6", "org.jfrog.test:multi1:3.7-SNAPSHOT", "org.jfrog.test:multi3:3.7-SNAPSHOT" ] ]
      } ]
    } ]
  }
  `), builds[0])
	assert.NoError(t, err)
	expected := "\n\n # Modules Published \n\n\n ### `jfrog-python-example` \n\n\n <pre>📦 jfrog-python-example\n├── <a href=dist/jfrog_python_example-1.0-py3-none-any.whl target=\"_blank\">jfrog_python_example-1.0-py3-none-any.whl</a>\n└── <a href=dist/jfrog_python_example-1.0.tar.gz target=\"_blank\">jfrog_python_example-1.0.tar.gz</a>\n\n</pre>\n ### `npm-example:0.0.3` \n\n\n <pre>📦 npm-example:0.0.3\n└── <a href=npm-example/-/npm-example-0.0.3.tgz target=\"_blank\">npm-example-0.0.3.tgz</a>\n\n</pre>\n ### `github.com/you/hello` \n\n\n <pre>📦 github.com\n└── 📁 you\n    └── 📁 hello\n        ├── 📄 v1.0.0.info\n        ├── 📄 v1.0.0.mod\n        └── 📄 v1.0.0.zip\n\n</pre>\n ### `generic-module` \n\n\n <pre>📦 generic-module\n└── <a href=deploy-file.sh target=\"_blank\">deploy-file.sh</a>\n\n</pre>\n ### `multiarch-image:1` \n\n\n <pre>📦 multiarch-image:1\n└── <a href=multiarch-image/1/list.manifest.json ta"
	assert.Equal(t, expected, gh.buildInfoModules(builds))
}

func TestParseBuildTime(t *testing.T) {
	// Test format
	actual := parseBuildTime("2006-01-02T15:04:05.000-0700")
	expected := "Jan 2, 2006 , 15:04:05"
	assert.Equal(t, expected, actual)
	// Test invalid format
	expected = "N/A"
	actual = parseBuildTime("")
	assert.Equal(t, expected, actual)
}
