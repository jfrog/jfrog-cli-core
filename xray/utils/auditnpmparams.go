package utils

type AuditNpmParams struct {
	AuditParams
	npmIgnoreNodeModules    bool
	npmOverwritePackageLock bool
}

func (anp AuditNpmParams) SetNpmIgnoreNodeModules(ignoreNpmNodeModules bool) AuditNpmParams {
	anp.npmIgnoreNodeModules = ignoreNpmNodeModules
	return anp
}

func (anp AuditNpmParams) SetNpmOverwritePackageLock(overwritePackageLock bool) AuditNpmParams {
	anp.npmOverwritePackageLock = overwritePackageLock
	return anp
}

func (anp AuditNpmParams) NpmIgnoreNodeModules() bool {
	return anp.npmIgnoreNodeModules
}

func (anp AuditNpmParams) NpmOverwritePackageLock() bool {
	return anp.npmOverwritePackageLock
}
