package formats

import (
	"strings"
)

func ConvertToVulnerabilityTableRow(rows []VulnerabilityOrViolationRow) (tableRows []VulnerabilityTableRow) {
	for i := range rows {
		tableRows = append(tableRows, VulnerabilityTableRow{
			Severity:                  rows[i].Severity,
			SeverityNumValue:          rows[i].SeverityNumValue,
			ImpactedDependencyName:    rows[i].ImpactedDependencyName,
			ImpactedDependencyVersion: rows[i].ImpactedDependencyVersion,
			ImpactedDependencyType:    rows[i].ImpactedDependencyType,
			FixedVersions:             strings.Join(rows[i].FixedVersions, "\n"),
			DirectDependencies:        ConvertToComponentTableRow(rows[i].Components),
			Cves:                      ConvertToCveTableRow(rows[i].Cves),
			IssueId:                   rows[i].IssueId,
		})
	}
	return
}

func ConvertToVulnerabilityScanTableRow(rows []VulnerabilityOrViolationRow) (tableRows []VulnerabilityScanTableRow) {
	for i := range rows {
		tableRows = append(tableRows, VulnerabilityScanTableRow{
			Severity:               rows[i].Severity,
			SeverityNumValue:       rows[i].SeverityNumValue,
			ImpactedPackageName:    rows[i].ImpactedDependencyName,
			ImpactedPackageVersion: rows[i].ImpactedDependencyVersion,
			ImpactedPackageType:    rows[i].ImpactedDependencyType,
			FixedVersions:          strings.Join(rows[i].FixedVersions, "\n"),
			DirectPackages:         ConvertToComponentScanTableRow(rows[i].Components),
			Cves:                   ConvertToCveTableRow(rows[i].Cves),
			IssueId:                rows[i].IssueId,
		})
	}
	return
}

func ConvertToLicenseViolationTableRow(rows []LicenseViolationRow) (tableRows []LicenseViolationTableRow) {
	for i := range rows {
		tableRows = append(tableRows, LicenseViolationTableRow{
			LicenseKey:                rows[i].LicenseKey,
			Severity:                  rows[i].Severity,
			SeverityNumValue:          rows[i].SeverityNumValue,
			ImpactedDependencyName:    rows[i].ImpactedDependencyName,
			ImpactedDependencyVersion: rows[i].ImpactedDependencyVersion,
			ImpactedDependencyType:    rows[i].ImpactedDependencyType,
			DirectDependencies:        ConvertToComponentTableRow(rows[i].Components),
		})
	}
	return
}

func ConvertToLicenseViolationScanTableRow(rows []LicenseViolationRow) (tableRows []LicenseViolationScanTableRow) {
	for i := range rows {
		tableRows = append(tableRows, LicenseViolationScanTableRow{
			LicenseKey:             rows[i].LicenseKey,
			Severity:               rows[i].Severity,
			SeverityNumValue:       rows[i].SeverityNumValue,
			ImpactedPackageName:    rows[i].ImpactedDependencyName,
			ImpactedPackageVersion: rows[i].ImpactedDependencyVersion,
			ImpactedDependencyType: rows[i].ImpactedDependencyType,
			DirectDependencies:     ConvertToComponentScanTableRow(rows[i].Components),
		})
	}
	return
}

func ConvertToLicenseTableRow(rows []LicenseRow) (tableRows []LicenseTableRow) {
	for i := range rows {
		tableRows = append(tableRows, LicenseTableRow{
			LicenseKey:                rows[i].LicenseKey,
			ImpactedDependencyName:    rows[i].ImpactedDependencyName,
			ImpactedDependencyVersion: rows[i].ImpactedDependencyVersion,
			ImpactedDependencyType:    rows[i].ImpactedDependencyType,
			DirectDependencies:        ConvertToComponentTableRow(rows[i].Components),
		})
	}
	return
}

func ConvertToLicenseScanTableRow(rows []LicenseRow) (tableRows []LicenseScanTableRow) {
	for i := range rows {
		tableRows = append(tableRows, LicenseScanTableRow{
			LicenseKey:             rows[i].LicenseKey,
			ImpactedPackageName:    rows[i].ImpactedDependencyName,
			ImpactedPackageVersion: rows[i].ImpactedDependencyVersion,
			ImpactedDependencyType: rows[i].ImpactedDependencyType,
			DirectDependencies:     ConvertToComponentScanTableRow(rows[i].Components),
		})
	}
	return
}

func ConvertToOperationalRiskViolationTableRow(rows []OperationalRiskViolationRow) (tableRows []OperationalRiskViolationTableRow) {
	for i := range rows {
		tableRows = append(tableRows, OperationalRiskViolationTableRow{
			Severity:                  rows[i].Severity,
			SeverityNumValue:          rows[i].SeverityNumValue,
			ImpactedDependencyName:    rows[i].ImpactedDependencyName,
			ImpactedDependencyVersion: rows[i].ImpactedDependencyVersion,
			ImpactedDependencyType:    rows[i].ImpactedDependencyType,
			DirectDependencies:        ConvertToComponentTableRow(rows[i].Components),
			IsEol:                     rows[i].IsEol,
			Cadence:                   rows[i].Cadence,
			Commits:                   rows[i].Commits,
			Committers:                rows[i].Committers,
			NewerVersions:             rows[i].NewerVersions,
			LatestVersion:             rows[i].LatestVersion,
			RiskReason:                rows[i].RiskReason,
			EolMessage:                rows[i].EolMessage,
		})
	}
	return
}

func ConvertToOperationalRiskViolationScanTableRow(rows []OperationalRiskViolationRow) (tableRows []OperationalRiskViolationScanTableRow) {
	for i := range rows {
		tableRows = append(tableRows, OperationalRiskViolationScanTableRow{
			Severity:               rows[i].Severity,
			SeverityNumValue:       rows[i].SeverityNumValue,
			ImpactedPackageName:    rows[i].ImpactedDependencyName,
			ImpactedPackageVersion: rows[i].ImpactedDependencyVersion,
			ImpactedDependencyType: rows[i].ImpactedDependencyType,
			DirectDependencies:     ConvertToComponentScanTableRow(rows[i].Components),
			IsEol:                  rows[i].IsEol,
			Cadence:                rows[i].Cadence,
			Commits:                rows[i].Commits,
			Committers:             rows[i].Committers,
			NewerVersions:          rows[i].NewerVersions,
			LatestVersion:          rows[i].LatestVersion,
			RiskReason:             rows[i].RiskReason,
			EolMessage:             rows[i].EolMessage,
		})
	}
	return
}

func ConvertToComponentTableRow(rows []ComponentRow) (tableRows []directDependenciesTableRow) {
	for i := range rows {
		tableRows = append(tableRows, directDependenciesTableRow{
			Name:    rows[i].Name,
			Version: rows[i].Version,
		})
	}
	return
}

func ConvertToComponentScanTableRow(rows []ComponentRow) (tableRows []directPackagesTableRow) {
	for i := range rows {
		tableRows = append(tableRows, directPackagesTableRow{
			Name:    rows[i].Name,
			Version: rows[i].Version,
		})
	}
	return
}

func ConvertToCveTableRow(rows []CveRow) (tableRows []cveTableRow) {
	for i := range rows {
		tableRows = append(tableRows, cveTableRow{
			Id:     rows[i].Id,
			CvssV2: rows[i].CvssV2,
			CvssV3: rows[i].CvssV3,
		})
	}
	return
}
