package formats

import "github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"

// Structs in this file should NOT be changed!
// The structs are used as an API for the simple-json format, thus changing their structure or the 'json' annotation will break the API.

// This struct holds the sorted results of the simple-json output.
type SimpleJsonResults struct {
	Vulnerabilities           []VulnerabilityOrViolationRow `json:"vulnerabilities"`
	SecurityViolations        []VulnerabilityOrViolationRow `json:"securityViolations"`
	LicensesViolations        []LicenseViolationRow         `json:"licensesViolations"`
	Licenses                  []LicenseRow                  `json:"licenses"`
	OperationalRiskViolations []OperationalRiskViolationRow `json:"operationalRiskViolations"`
	Secrets                   []SourceCodeRow               `json:"secrets"`
	Iacs                      []SourceCodeRow               `json:"iacViolations"`
	Sast                      []SourceCodeRow               `json:"sastViolations"`
	Errors                    []SimpleJsonError             `json:"errors"`
}

// Used for vulnerabilities and security violations
type VulnerabilityOrViolationRow struct {
	Summary                   string                    `json:"summary"`
	Severity                  string                    `json:"severity"`
	Applicable                string                    `json:"applicable"`
	SeverityNumValue          int                       `json:"-"` // For sorting
	ImpactedDependencyName    string                    `json:"impactedPackageName"`
	ImpactedDependencyVersion string                    `json:"impactedPackageVersion"`
	ImpactedDependencyType    string                    `json:"impactedPackageType"`
	FixedVersions             []string                  `json:"fixedVersions"`
	Components                []ComponentRow            `json:"components"`
	Cves                      []CveRow                  `json:"cves"`
	IssueId                   string                    `json:"issueId"`
	References                []string                  `json:"references"`
	ImpactPaths               [][]ComponentRow          `json:"impactPaths"`
	JfrogResearchInformation  *JfrogResearchInformation `json:"jfrogResearchInformation"`
	Technology                coreutils.Technology      `json:"-"`
}

type LicenseRow struct {
	LicenseKey                string           `json:"licenseKey"`
	ImpactedDependencyName    string           `json:"impactedPackageName"`
	ImpactedDependencyVersion string           `json:"impactedPackageVersion"`
	ImpactedDependencyType    string           `json:"impactedPackageType"`
	Components                []ComponentRow   `json:"components"`
	ImpactPaths               [][]ComponentRow `json:"impactPaths"`
}

type LicenseViolationRow struct {
	LicenseKey                string         `json:"licenseKey"`
	Severity                  string         `json:"severity"`
	Applicable                string         `json:"applicable"`
	SeverityNumValue          int            `json:"-"` // For sorting
	ImpactedDependencyName    string         `json:"impactedPackageName"`
	ImpactedDependencyVersion string         `json:"impactedPackageVersion"`
	ImpactedDependencyType    string         `json:"impactedPackageType"`
	Components                []ComponentRow `json:"components"`
}

type OperationalRiskViolationRow struct {
	Severity                  string         `json:"severity"`
	SeverityNumValue          int            `json:"-"` // For sorting
	ImpactedDependencyName    string         `json:"impactedPackageName"`
	ImpactedDependencyVersion string         `json:"impactedPackageVersion"`
	ImpactedDependencyType    string         `json:"impactedPackageType"`
	Components                []ComponentRow `json:"components"`
	RiskReason                string         `json:"riskReason"`
	IsEol                     string         `json:"isEndOfLife"`
	EolMessage                string         `json:"endOfLifeMessage"`
	Cadence                   string         `json:"cadence"`
	Commits                   string         `json:"commits"`
	Committers                string         `json:"committers"`
	NewerVersions             string         `json:"newerVersions"`
	LatestVersion             string         `json:"latestVersion"`
}

type SourceCodeRow struct {
	Severity         string `json:"severity"`
	SeverityNumValue int    `json:"-"` // For sorting
	SourceCodeLocationRow
	Type     string                    `json:"type"`
	CodeFlow [][]SourceCodeLocationRow `json:"codeFlow,omitempty"`
}

type SourceCodeLocationRow struct {
	File       string `json:"file"`
	LineColumn string `json:"lineColumn"`
	Snippet    string `json:"snippet"`
}

type ComponentRow struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type CveRow struct {
	Id            string         `json:"id"`
	CvssV2        string         `json:"cvssV2"`
	CvssV3        string         `json:"cvssV3"`
	Applicability *Applicability `json:"applicability,omitempty"`
}

type Applicability struct {
	Status             bool       `json:"status"`
	ScannerDescription string     `json:"scannerDescription,omitempty"`
	Evidence           []Evidence `json:"evidence,omitempty"`
}

type Evidence struct {
	SourceCodeLocationRow
	Reason string `json:"reason,omitempty"`
}

type SimpleJsonError struct {
	FilePath     string `json:"filePath"`
	ErrorMessage string `json:"errorMessage"`
}

type JfrogResearchInformation struct {
	Summary         string                        `json:"summary,omitempty"`
	Details         string                        `json:"details,omitempty"`
	Severity        string                        `json:"severity,omitempty"`
	SeverityReasons []JfrogResearchSeverityReason `json:"severityReasons,omitempty"`
	Remediation     string                        `json:"remediation,omitempty"`
}

type JfrogResearchSeverityReason struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	IsPositive  bool   `json:"isPositive,omitempty"`
}
