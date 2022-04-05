package formats

// Structs in this file are used for the 'table' format output of scan/audit commands.
// Annotations are as described in the tableutils.PrintTable description.
// Use the conversion methods in this package to convert from the API structs to the table structs.

// Used for vulnerabilities and security violations
type VulnerabilityTableRow struct {
	Severity               string              `col-name:"Severity"`
	SeverityNumValue       int                 // For sorting
	ImpactedPackageName    string              `col-name:"Impacted\nPackage"`
	ImpactedPackageVersion string              `col-name:"Impacted\nPackage\nVersion"`
	ImpactedPackageType    string              `col-name:"Type"`
	FixedVersions          string              `col-name:"Fixed\nVersions"`
	Components             []ComponentTableRow `embed-table:"true"`
	Cves                   []CveTableRow       `embed-table:"true"`
	IssueId                string              `col-name:"Issue ID" extended:"true"`
}

type LicenseTableRow struct {
	LicenseKey             string              `col-name:"License"`
	ImpactedPackageName    string              `col-name:"Impacted\nPackage"`
	ImpactedPackageVersion string              `col-name:"Impacted\nPackage\nVersion"`
	ImpactedPackageType    string              `col-name:"Type"`
	Components             []ComponentTableRow `embed-table:"true"`
}

type LicenseViolationTableRow struct {
	LicenseKey             string              `col-name:"License"`
	Severity               string              `col-name:"Severity"`
	SeverityNumValue       int                 // For sorting
	ImpactedPackageName    string              `col-name:"Impacted\nPackage"`
	ImpactedPackageVersion string              `col-name:"Impacted\nPackage\nVersion"`
	ImpactedPackageType    string              `col-name:"Type"`
	Components             []ComponentTableRow `embed-table:"true"`
}

type OperationalRiskViolationTableRow struct {
	Severity               string              `col-name:"Severity"`
	SeverityNumValue       int                 // For sorting
	ImpactedPackageName    string              `col-name:"Impacted\nPackage"`
	ImpactedPackageVersion string              `col-name:"Impacted\nPackage\nVersion"`
	ImpactedPackageType    string              `col-name:"Type"`
	Components             []ComponentTableRow `embed-table:"true"`
	RiskReason             string              `col-name:"Risk\nReason"`
	IsEol                  string              `col-name:"Is\nEnd\nOf\nLife" extended:"true"`
	EolMessage             string              `col-name:"End\nOf\nLife\nMessage" extended:"true"`
	Cadence                string              `col-name:"Cadence"  extended:"true"`
	Commits                string              `col-name:"Commits"  extended:"true"`
	Committers             string              `col-name:"Committers"  extended:"true"`
	NewerVersions          string              `col-name:"Newer\nVersions" extended:"true"`
	LatestVersion          string              `col-name:"Latest\nVersion" extended:"true"`
}

type ComponentTableRow struct {
	Name    string `col-name:"Component"`
	Version string `col-name:"Component\nVersion"`
}

type CveTableRow struct {
	Id     string `col-name:"CVE"`
	CvssV2 string `col-name:"CVSS\nv2" extended:"true"`
	CvssV3 string `col-name:"CVSS\nv3" extended:"true"`
}
