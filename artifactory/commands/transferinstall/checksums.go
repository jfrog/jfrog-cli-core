package transferinstall

// The version installed when no version is requested. Must be a key of pluginChecksums.
const latestPluginVersion = "1.7.6"

// SHA-256 digests of the data-transfer plugin files published to releases.jfrog.io, by plugin version and file name.
// Downloaded plugin files are installed only if they match these digests, so a new plugin release must be added here.
var pluginChecksums = map[string]map[string]string{
	"1.0.2": pluginFileChecksums("b14464caaca103cc782dbaf065446dc4ae1951256dd165516d66bac95b45a31b", "8ceb036cc1e251dfca56e2f182944d9a85f5b58470ad184ec9a842fe19032a82"),
	"1.0.3": pluginFileChecksums("b14464caaca103cc782dbaf065446dc4ae1951256dd165516d66bac95b45a31b", "ff349da086aa36ec786cea52363dfdb11ea35ab0d3ec69e00b0c592697c99e45"),
	"1.1.0": pluginFileChecksums("6661691f07c3ff3bb7db46ebaf55bc5fd5aec2e2aa8837e56d958c2377ddb766", "f79d8d8fe2329992278e29b4e9c5708c504f4450c7bf2093cc593aea09aa047e"),
	"1.2.0": pluginFileChecksums("06788274bf8ae1df9acbe7e6891965ae365d486431bd9106bbb40f85d994e776", "5b3a596313a1bb5a0b72782401eb49c71d3763086d9b5826ca58290090367f29"),
	"1.2.1": pluginFileChecksums("06788274bf8ae1df9acbe7e6891965ae365d486431bd9106bbb40f85d994e776", "1429dde27b202e8b2d58a37ad80d76232df2e765321c83a42e9a93f934e1eb50"),
	"1.2.2": pluginFileChecksums("06788274bf8ae1df9acbe7e6891965ae365d486431bd9106bbb40f85d994e776", "7256cf87cb4412cdca6239771b550102254c83f01a4b59f46ceff3dd811504aa"),
	"1.3.0": pluginFileChecksums("06788274bf8ae1df9acbe7e6891965ae365d486431bd9106bbb40f85d994e776", "641ce476abc43c59ee6a37ccd99afb610ca824526814e48361149b3844e67316"),
	"1.3.1": pluginFileChecksums("06788274bf8ae1df9acbe7e6891965ae365d486431bd9106bbb40f85d994e776", "f854f82beffcadfbfb269d5dabf2c183361776e1598d59e5458e7736e284bc15"),
	"1.3.2": pluginFileChecksums("06788274bf8ae1df9acbe7e6891965ae365d486431bd9106bbb40f85d994e776", "3114ee381b758c3c440ff0df6ca3642f0390bb36d5b72d405d5cad168927b2dc"),
	"1.3.3": pluginFileChecksums("06788274bf8ae1df9acbe7e6891965ae365d486431bd9106bbb40f85d994e776", "1be2bea07cb927c9e443ec0416fa48bf4ba7f2084615a4034514dee42c3c8286"),
	"1.3.4": pluginFileChecksums("06788274bf8ae1df9acbe7e6891965ae365d486431bd9106bbb40f85d994e776", "2c68d7510a63ac39713a3e973745f002cfa2b2a0cbfc4fb9f18b7a5f0dd5eee0"),
	"1.4.0": pluginFileChecksums("0157c1dd53cd3e9495d1ab1e795eb92a40a19bc2f0e5b8622b19cb71d89ab934", "aa35eb950a27694165659097a0b84574f921a42ceef993c5e00bc21aed66bf1a"),
	"1.4.1": pluginFileChecksums("0157c1dd53cd3e9495d1ab1e795eb92a40a19bc2f0e5b8622b19cb71d89ab934", "43296252e8a6ed8b1e0731f8898c9a864088cee4d8f20f888c065068c954f8ea"),
	"1.5.0": pluginFileChecksums("f0c8971ab338d82a5fee6df8db21ea7a40675826c893ae267a14e616aefe8f7f", "83566c7ee902623ecf4c8bfc63ff3e93d2af6b44be5753477bb9b45ba61a45a1"),
	"1.6.0": pluginFileChecksums("f0c8971ab338d82a5fee6df8db21ea7a40675826c893ae267a14e616aefe8f7f", "afc8122e5ed15677dd754dcbf6be2d09234247ebab2f0ba545c95f73c6288b64"),
	"1.6.1": pluginFileChecksums("f0c8971ab338d82a5fee6df8db21ea7a40675826c893ae267a14e616aefe8f7f", "46d37fcbdaa87196eba124865d429710c0cce4cd68715f03b853a8872ebf14d8"),
	"1.7.0": pluginFileChecksums("f0c8971ab338d82a5fee6df8db21ea7a40675826c893ae267a14e616aefe8f7f", "3dad861e629c5fa391e146ed89cf754289f888df155146d1b9903a9dc2c8a273"),
	"1.7.1": pluginFileChecksums("f0c8971ab338d82a5fee6df8db21ea7a40675826c893ae267a14e616aefe8f7f", "a657156033a621865d77a6181d9750de04cc31b38f2f7d89449bfb6ade835210"),
	"1.7.2": pluginFileChecksums("f0c8971ab338d82a5fee6df8db21ea7a40675826c893ae267a14e616aefe8f7f", "6a06348ce201e03a60897a90f3c356649f923fc8e9c0ceb29f868b626a37eb18"),
	"1.7.3": pluginFileChecksums("f0c8971ab338d82a5fee6df8db21ea7a40675826c893ae267a14e616aefe8f7f", "f8311d7e304a752d216672249beb2312c22358e3174bd3a9484b429e30418b15"),
	"1.7.4": pluginFileChecksums("f0c8971ab338d82a5fee6df8db21ea7a40675826c893ae267a14e616aefe8f7f", "282c81ef777bf18b8a1ac8702c327be36800e59beef22ce064582756cd0e0f78"),
	"1.7.5": pluginFileChecksums("f0c8971ab338d82a5fee6df8db21ea7a40675826c893ae267a14e616aefe8f7f", "675b3aee34eff126b8941ee62d16a5b2f1246c073fc50026bb1f4e09411e4d2f"),
	"1.7.6": pluginFileChecksums("f0c8971ab338d82a5fee6df8db21ea7a40675826c893ae267a14e616aefe8f7f", "1e773e8de449c949d58bb9cba0b84a797d14da87ca7b05ffd2cc741932ce35a4"),
}

func pluginFileChecksums(groovySha256, jarSha256 string) map[string]string {
	return map[string]string{
		groovyFileName: groovySha256,
		jarFileName:    jarSha256,
	}
}
