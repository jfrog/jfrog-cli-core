package formats

// Structs in this file should NOT be changed!
// The structs are used as an API for the simple-json format, thus changing their structure or the 'json' annotation will break the API.

// This struct holds the sorted results of the simple-json output.
type SimpleJsonResults struct {
	Vulnerabilities           []VulnerabilityOrViolationRow `json:"vulnerabilities"`
	SecurityViolations        []VulnerabilityOrViolationRow `json:"securityViolations"`
	LicensesViolations        []LicenseViolationRow         `json:"licensesViolations"`
	Licenses                  []LicenseRow                  `json:"licenses"`
	OperationalRiskViolations []OperationalRiskViolationRow `json:"operationalRiskViolations"`
}

// Used for vulnerabilities and security violations
type VulnerabilityOrViolationRow struct {
	Summary                string         `json:"summary"`
	Severity               string         `json:"severity"`
	SeverityNumValue       int            `json:"-"` // For sorting
	ImpactedPackageName    string         `json:"impactedPackageName"`
	ImpactedPackageVersion string         `json:"impactedPackageVersion"`
	ImpactedPackageType    string         `json:"impactedPackageType"`
	FixedVersions          []string       `json:"fixedVersions"`
	Components             []ComponentRow `json:"components"`
	Cves                   []CveRow       `json:"cves"`
	IssueId                string         `json:"issueId"`
	References             []string       `json:"references"`
}

type LicenseRow struct {
	LicenseKey             string         `json:"licenseKey"`
	ImpactedPackageName    string         `json:"impactedPackageName"`
	ImpactedPackageVersion string         `json:"impactedPackageVersion"`
	ImpactedPackageType    string         `json:"impactedPackageType"`
	Components             []ComponentRow `json:"components"`
}

type LicenseViolationRow struct {
	LicenseKey             string         `json:"licenseKey"`
	Severity               string         `json:"severity"`
	SeverityNumValue       int            `json:"-"` // For sorting
	ImpactedPackageName    string         `json:"impactedPackageName"`
	ImpactedPackageVersion string         `json:"impactedPackageVersion"`
	ImpactedPackageType    string         `json:"impactedPackageType"`
	Components             []ComponentRow `json:"components"`
}

type OperationalRiskViolationRow struct {
	Severity               string         `json:"severity"`
	SeverityNumValue       int            `json:"-"` // For sorting
	ImpactedPackageName    string         `json:"impactedPackageName"`
	ImpactedPackageVersion string         `json:"impactedPackageVersion"`
	ImpactedPackageType    string         `json:"impactedPackageType"`
	Components             []ComponentRow `json:"components"`
	RiskReason             string         `json:"riskReason"`
	IsEol                  string         `json:"isEndOfLife"`
	EolMessage             string         `json:"endOfLifeMessage"`
	Cadence                string         `json:"cadence"`
	Commits                string         `json:"commits"`
	Committers             string         `json:"committers"`
	NewerVersions          string         `json:"newerVersions"`
	LatestVersion          string         `json:"latestVersion"`
}

type ComponentRow struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type CveRow struct {
	Id     string `json:"id"`
	CvssV2 string `json:"cvssV2"`
	CvssV3 string `json:"cvssV3"`
}
