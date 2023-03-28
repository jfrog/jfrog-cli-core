package formats

// Structs in this file are used for the 'table' format output of scan/audit commands.
// Annotations are as described in the tableutils.PrintTable description.
// Use the conversion methods in this package to convert from the API structs to the table structs.

// Used for vulnerabilities and security violations
type vulnerabilityTableRow struct {
	Severity string `col-name:"Severity"`
	// For sorting
	severityNumValue          int
	directDependencies        []directDependenciesTableRow `embed-table:"true"`
	impactedDependencyName    string                       `col-name:"Impacted\nDependency\nName"`
	impactedDependencyVersion string                       `col-name:"Impacted\nDependency\nVersion"`
	FixedVersions             string                       `col-name:"Fixed\nVersions"`
	impactedDependencyType    string                       `col-name:"Type"`
	Cves                      []cveTableRow                `embed-table:"true"`
	IssueId                   string                       `col-name:"Issue ID" extended:"true"`
}

type vulnerabilityScanTableRow struct {
	Severity string `col-name:"Severity"`
	// For sorting
	severityNumValue       int
	directPackages         []directPackagesTableRow `embed-table:"true"`
	impactedPackageName    string                   `col-name:"Impacted\nPackage\nName"`
	impactedPackageVersion string                   `col-name:"Impacted\nPackage\nVersion"`
	fixedVersions          string                   `col-name:"Fixed\nVersions"`
	ImpactedPackageType    string                   `col-name:"Type"`
	cves                   []cveTableRow            `embed-table:"true"`
	IssueId                string                   `col-name:"Issue ID" extended:"true"`
}

type licenseTableRow struct {
	licenseKey                string                       `col-name:"License"`
	directDependencies        []directDependenciesTableRow `embed-table:"true"`
	impactedDependencyName    string                       `col-name:"Impacted\nDependency"`
	impactedDependencyVersion string                       `col-name:"Impacted\nDependency\nVersion"`
	impactedDependencyType    string                       `col-name:"Type"`
}

type licenseScanTableRow struct {
	licenseKey             string                   `col-name:"License"`
	directDependencies     []directPackagesTableRow `embed-table:"true"`
	impactedPackageName    string                   `col-name:"Impacted\nPackage"`
	impactedPackageVersion string                   `col-name:"Impacted\nPackage\nVersion"`
	impactedDependencyType string                   `col-name:"Type"`
}

type licenseViolationTableRow struct {
	LicenseKey string `col-name:"License"`
	Severity   string `col-name:"Severity"`
	// For sorting
	severityNumValue          int
	directDependencies        []directDependenciesTableRow `embed-table:"true"`
	impactedDependencyName    string                       `col-name:"Impacted\nDependency"`
	impactedDependencyVersion string                       `col-name:"Impacted\nDependency\nVersion"`
	impactedDependencyType    string                       `col-name:"Type"`
}

type licenseViolationScanTableRow struct {
	licenseKey string `col-name:"License"`
	Severity   string `col-name:"Severity"`
	// For sorting
	severityNumValue       int
	directDependencies     []directPackagesTableRow `embed-table:"true"`
	impactedPackageName    string                   `col-name:"Impacted\nPackage"`
	impactedPackageVersion string                   `col-name:"Impacted\nPackage\nVersion"`
	impactedDependencyType string                   `col-name:"Type"`
}

type operationalRiskViolationTableRow struct {
	Severity string `col-name:"Severity"`
	// For sorting
	severityNumValue          int
	directDependencies        []directDependenciesTableRow `embed-table:"true"`
	impactedDependencyName    string                       `col-name:"Impacted\nDependency"`
	impactedDependencyVersion string                       `col-name:"Impacted\nDependency\nVersion"`
	impactedDependencyType    string                       `col-name:"Type"`
	riskReason                string                       `col-name:"Risk\nReason"`
	isEol                     string                       `col-name:"Is\nEnd\nOf\nLife" extended:"true"`
	eolMessage                string                       `col-name:"End\nOf\nLife\nMessage" extended:"true"`
	Cadence                   string                       `col-name:"Cadence"  extended:"true"`
	Commits                   string                       `col-name:"Commits"  extended:"true"`
	Committers                string                       `col-name:"Committers"  extended:"true"`
	NewerVersions             string                       `col-name:"Newer\nVersions" extended:"true"`
	LatestVersion             string                       `col-name:"Latest\nVersion" extended:"true"`
}

type operationalRiskViolationScanTableRow struct {
	Severity string `col-name:"Severity"`
	// For sorting
	severityNumValue       int
	directDependencies     []directPackagesTableRow `embed-table:"true"`
	impactedPackageName    string                   `col-name:"Impacted\nPackage"`
	impactedPackageVersion string                   `col-name:"Impacted\nPackage\nVersion"`
	impactedDependencyType string                   `col-name:"Type"`
	riskReason             string                   `col-name:"Risk\nReason"`
	isEol                  string                   `col-name:"Is\nEnd\nOf\nLife" extended:"true"`
	eolMessage             string                   `col-name:"End\nOf\nLife\nMessage" extended:"true"`
	Cadence                string                   `col-name:"Cadence"  extended:"true"`
	Commits                string                   `col-name:"Commits"  extended:"true"`
	Committers             string                   `col-name:"Committers"  extended:"true"`
	NewerVersions          string                   `col-name:"Newer\nVersions" extended:"true"`
	LatestVersion          string                   `col-name:"Latest\nVersion" extended:"true"`
}

type directDependenciesTableRow struct {
	Name    string `col-name:"Direct\nDependency"`
	Version string `col-name:"Direct\nDependency\nVersion"`
}

type directPackagesTableRow struct {
	Name    string `col-name:"Direct\nPackage"`
	Version string `col-name:"Direct\nPackage\nVersion"`
}

type cveTableRow struct {
	Id     string `col-name:"CVE"`
	CvssV2 string `col-name:"CVSS\nv2" extended:"true"`
	CvssV3 string `col-name:"CVSS\nv3" extended:"true"`
}
