package utils

type AuditNpmParams struct {
	AuditParams
	npmIgnoreNodeModules    bool
	npmOverWritePackageLock bool
}

func (abp AuditNpmParams) SetNpmIgnoreNodeModules(ignoreNpmNodeModules bool) AuditNpmParams {
	abp.npmIgnoreNodeModules = ignoreNpmNodeModules
	return abp
}

func (abp AuditNpmParams) SetNpmOverwritePackageLock(overwritePackageLock bool) AuditNpmParams {
	abp.npmOverWritePackageLock = overwritePackageLock
	return abp
}

func (abp AuditNpmParams) NpmIgnoreNodeModules() bool {
	return abp.npmIgnoreNodeModules
}

func (abp AuditNpmParams) NpmOverwritePackageLock() bool {
	return abp.npmOverWritePackageLock
}
