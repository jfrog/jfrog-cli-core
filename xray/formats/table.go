package formats

// Structs in this file are used for the 'table' format output of scan/audit commands.
// Annotations are as described in the tableutils.PrintTable description.
// Use the conversion methods in this package to convert from the API structs to the table structs.

// Used for vulnerabilities and security violations
type VulnerabilityTableRow struct {
	Severity                  string                       `col-name:"Severity"`
	SeverityNumValue          int                          // For sorting
	DirectDependencies        []DirectDependenciesTableRow `embed-table:"true"`
	ImpactedDependencyName    string                       `col-name:"Impacted\nDependency\nName"`
	ImpactedDependencyVersion string                       `col-name:"Impacted\nDependency\nVersion"`
	FixedVersions             string                       `col-name:"Fixed\nVersions"`
	ImpactedDependencyType    string                       `col-name:"Type"`
	Cves                      []CveTableRow                `embed-table:"true"`
	IssueId                   string                       `col-name:"Issue ID" extended:"true"`
}

type LicenseTableRow struct {
	LicenseKey                string                       `col-name:"License"`
	DirectDependencies        []DirectDependenciesTableRow `embed-table:"true"`
	ImpactedDependencyName    string                       `col-name:"Impacted\nDependency"`
	ImpactedDependencyVersion string                       `col-name:"Impacted\nDependency\nVersion"`
	ImpactedDependencyType    string                       `col-name:"Type"`
}

type LicenseViolationTableRow struct {
	LicenseKey                string                       `col-name:"License"`
	Severity                  string                       `col-name:"Severity"`
	SeverityNumValue          int                          // For sorting
	DirectDependencies        []DirectDependenciesTableRow `embed-table:"true"`
	ImpactedDependencyName    string                       `col-name:"Impacted\nDependency"`
	ImpactedDependencyVersion string                       `col-name:"Impacted\nDependency\nVersion"`
	ImpactedDependencyType    string                       `col-name:"Type"`
}

type OperationalRiskViolationTableRow struct {
	Severity                  string                       `col-name:"Severity"`
	SeverityNumValue          int                          // For sorting
	DirectDependencies        []DirectDependenciesTableRow `embed-table:"true"`
	ImpactedDependencyName    string                       `col-name:"Impacted\nDependency"`
	ImpactedDependencyVersion string                       `col-name:"Impacted\nDependency\nVersion"`
	ImpactedDependencyType    string                       `col-name:"Type"`
	RiskReason                string                       `col-name:"Risk\nReason"`
	IsEol                     string                       `col-name:"Is\nEnd\nOf\nLife" extended:"true"`
	EolMessage                string                       `col-name:"End\nOf\nLife\nMessage" extended:"true"`
	Cadence                   string                       `col-name:"Cadence"  extended:"true"`
	Commits                   string                       `col-name:"Commits"  extended:"true"`
	Committers                string                       `col-name:"Committers"  extended:"true"`
	NewerVersions             string                       `col-name:"Newer\nVersions" extended:"true"`
	LatestVersion             string                       `col-name:"Latest\nVersion" extended:"true"`
}

type DirectDependenciesTableRow struct {
	Name    string `col-name:"Direct\nDependency"`
	Version string `col-name:"Direct\nDependency\nVersion"`
}

type CveTableRow struct {
	Id     string `col-name:"CVE"`
	CvssV2 string `col-name:"CVSS\nv2" extended:"true"`
	CvssV3 string `col-name:"CVSS\nv3" extended:"true"`
}
