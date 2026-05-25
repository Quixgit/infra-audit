package rules

import (
	"encoding/json"
	"fmt"

	awsscanner "infra-audit/internal/scanner/aws"
	"infra-audit/internal/model"
)

// EvaluateAWS generates security findings for an AWS inventory.
func EvaluateAWS(inv model.Inventory) []model.Finding {
	var findings []model.Finding

	// ── Security Groups ────────────────────────────────────────────────────────
	if raw, ok := inv.Resources["security_groups"]; ok {
		b, _ := json.Marshal(raw)
		var sgs []awsscanner.SecurityGroup
		if err := json.Unmarshal(b, &sgs); err == nil {
			for _, sg := range sgs {
				if sg.OpenToWorld {
					name := sg.GroupName
					if name == "" {
						name = sg.GroupID
					}
					severity := "high"
					ports := ""
					if len(sg.CriticalPorts) > 0 {
						severity = "critical"
						ports = fmt.Sprintf(" (exposed critical ports: %v)", sg.CriticalPorts)
					}
					findings = append(findings, model.Finding{
						ID:           "aws-sg-open-world",
						Title:        "Security group open to 0.0.0.0/0" + ports,
						Severity:     severity,
						Status:       "open",
						Category:     "Network Security",
						ResourceType: "SecurityGroup",
						ResourceName: name,
						ResourceID:   sg.GroupID,
						Standard:     "CIS AWS Foundations Benchmark",
						ControlMapping: []string{"CIS 5.1", "CIS 5.2", "NIST AC-4"},
						Risk:         "Unrestricted inbound access exposes instances to the internet, enabling unauthorized access and attacks.",
						BusinessImpact: "Potential data breach, ransomware, or lateral movement by attackers.",
						Recommendation: "Restrict inbound rules to specific trusted IP ranges. Never use 0.0.0.0/0 for SSH (22), RDP (3389), or database ports.",
						Remediation:  "1. Navigate to EC2 → Security Groups\n2. Select the security group\n3. Edit inbound rules\n4. Replace 0.0.0.0/0 with specific CIDR ranges\n5. For SSH, use a bastion host or VPN",
						Validation:   "Verify using: aws ec2 describe-security-groups --group-ids " + sg.GroupID,
						Priority:     "P1",
						Timeline:     "Immediate",
					})
				}
			}
		}
	}

	// ── EC2 Instances ──────────────────────────────────────────────────────────
	if raw, ok := inv.Resources["ec2_instances"]; ok {
		b, _ := json.Marshal(raw)
		var instances []awsscanner.EC2Instance
		if err := json.Unmarshal(b, &instances); err == nil {
			for _, inst := range instances {
				name := inst.Name
				if name == "" {
					name = inst.InstanceID
				}
				// IMDSv2 not enforced
				if !inst.IMDSv2Required {
					findings = append(findings, model.Finding{
						ID:           "aws-ec2-imdsv1",
						Title:        "EC2 instance uses IMDSv1 (metadata service vulnerable to SSRF)",
						Severity:     "high",
						Status:       "open",
						Category:     "Instance Security",
						ResourceType: "EC2Instance",
						ResourceName: name,
						ResourceID:   inst.InstanceID,
						Standard:     "CIS AWS Foundations Benchmark",
						ControlMapping: []string{"CIS 5.6", "NIST SI-3"},
						Risk:         "IMDSv1 allows any process on the instance to read AWS credentials via SSRF attacks.",
						BusinessImpact: "Attackers can steal IAM credentials and escalate privileges across AWS resources.",
						Recommendation: "Enforce IMDSv2 by requiring HTTP token in metadata requests.",
						Remediation:  "aws ec2 modify-instance-metadata-options --instance-id " + inst.InstanceID + " --http-tokens required --http-endpoint enabled",
						Validation:   "aws ec2 describe-instances --instance-ids " + inst.InstanceID + " --query 'Reservations[].Instances[].MetadataOptions'",
						Priority:     "P1",
						Timeline:     "This week",
					})
				}
				// No monitoring
				if !inst.MonitoringEnabled {
					findings = append(findings, model.Finding{
						ID:           "aws-ec2-no-monitoring",
						Title:        "EC2 instance detailed monitoring disabled",
						Severity:     "low",
						Status:       "open",
						Category:     "Monitoring",
						ResourceType: "EC2Instance",
						ResourceName: name,
						ResourceID:   inst.InstanceID,
						Standard:     "CIS AWS Foundations Benchmark",
						ControlMapping: []string{"CIS 3.1", "NIST AU-2"},
						Risk:         "Without detailed monitoring, security events and performance anomalies may go undetected.",
						Recommendation: "Enable detailed CloudWatch monitoring for all EC2 instances.",
						Remediation:  "aws ec2 monitor-instances --instance-ids " + inst.InstanceID,
						Priority:     "P3",
						Timeline:     "This month",
					})
				}
			}
		}
	}

	// ── VPCs ───────────────────────────────────────────────────────────────────
	if raw, ok := inv.Resources["vpcs"]; ok {
		b, _ := json.Marshal(raw)
		var vpcs []awsscanner.VPC
		if err := json.Unmarshal(b, &vpcs); err == nil {
			for _, vpc := range vpcs {
				if !vpc.FlowLogs {
					name := vpc.Name
					if name == "" {
						name = vpc.VpcID
					}
					findings = append(findings, model.Finding{
						ID:           "aws-vpc-no-flow-logs",
						Title:        "VPC flow logs disabled — network traffic not logged",
						Severity:     "medium",
						Status:       "open",
						Category:     "Logging & Monitoring",
						ResourceType: "VPC",
						ResourceName: name,
						ResourceID:   vpc.VpcID,
						Standard:     "CIS AWS Foundations Benchmark",
						ControlMapping: []string{"CIS 3.9", "NIST AU-12"},
						Risk:         "Without flow logs, network anomalies, data exfiltration, and attack patterns cannot be investigated.",
						BusinessImpact: "Compromised incident response capability; inability to detect or investigate breaches.",
						Recommendation: "Enable VPC flow logs to CloudWatch Logs or S3 for all VPCs.",
						Remediation:  "aws ec2 create-flow-logs --resource-type VPC --resource-ids " + vpc.VpcID + " --traffic-type ALL --log-destination-type cloud-watch-logs --log-group-name /aws/vpc/flowlogs",
						Priority:     "P2",
						Timeline:     "This week",
					})
				}
			}
		}
	}

	// ── S3 Buckets ─────────────────────────────────────────────────────────────
	if raw, ok := inv.Resources["s3_buckets"]; ok {
		b, _ := json.Marshal(raw)
		var buckets []awsscanner.S3Bucket
		if err := json.Unmarshal(b, &buckets); err == nil {
			for _, bucket := range buckets {
				// No block public access
				if !bucket.BlockPublicAccess {
					findings = append(findings, model.Finding{
						ID:           "aws-s3-public-access",
						Title:        "S3 bucket has public access block disabled",
						Severity:     "high",
						Status:       "open",
						Category:     "Data Security",
						ResourceType: "S3Bucket",
						ResourceName: bucket.Name,
						ResourceID:   bucket.Name,
						Standard:     "CIS AWS Foundations Benchmark",
						ControlMapping: []string{"CIS 2.1.2", "NIST AC-3", "SOC2 CC6.1"},
						Risk:         "Bucket may be publicly readable, allowing data exposure to the internet.",
						BusinessImpact: "Potential data breach, regulatory violations (GDPR, HIPAA), reputational damage.",
						Recommendation: "Enable S3 Block Public Access settings at both bucket and account level.",
						Remediation:  "aws s3api put-public-access-block --bucket " + bucket.Name + " --public-access-block-configuration BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true",
						Validation:   "aws s3api get-public-access-block --bucket " + bucket.Name,
						Priority:     "P1",
						Timeline:     "Immediate",
					})
				}
				// No encryption
				if !bucket.EncryptionEnabled {
					findings = append(findings, model.Finding{
						ID:           "aws-s3-no-encryption",
						Title:        "S3 bucket server-side encryption not enabled",
						Severity:     "medium",
						Status:       "open",
						Category:     "Data Security",
						ResourceType: "S3Bucket",
						ResourceName: bucket.Name,
						ResourceID:   bucket.Name,
						Standard:     "CIS AWS Foundations Benchmark",
						ControlMapping: []string{"CIS 2.1.1", "NIST SC-28"},
						Risk:         "Data at rest is not encrypted, increasing exposure if storage is compromised.",
						Recommendation: "Enable AES-256 or AWS KMS encryption for all S3 buckets.",
						Remediation:  "aws s3api put-bucket-encryption --bucket " + bucket.Name + " --server-side-encryption-configuration '{\"Rules\":[{\"ApplyServerSideEncryptionByDefault\":{\"SSEAlgorithm\":\"AES256\"}}]}'",
						Priority:     "P2",
						Timeline:     "This week",
					})
				}
				// No versioning
				if !bucket.VersioningEnabled {
					findings = append(findings, model.Finding{
						ID:           "aws-s3-no-versioning",
						Title:        "S3 bucket versioning disabled — data loss risk",
						Severity:     "low",
						Status:       "open",
						Category:     "Data Protection",
						ResourceType: "S3Bucket",
						ResourceName: bucket.Name,
						ResourceID:   bucket.Name,
						Standard:     "CIS AWS Foundations Benchmark",
						ControlMapping: []string{"CIS 2.1.3", "NIST CP-9"},
						Risk:         "Accidental or malicious deletions cannot be recovered.",
						Recommendation: "Enable S3 versioning to protect against accidental deletions and overwrites.",
						Remediation:  "aws s3api put-bucket-versioning --bucket " + bucket.Name + " --versioning-configuration Status=Enabled",
						Priority:     "P3",
						Timeline:     "This month",
					})
				}
				// No logging
				if !bucket.LoggingEnabled {
					findings = append(findings, model.Finding{
						ID:           "aws-s3-no-logging",
						Title:        "S3 bucket access logging disabled",
						Severity:     "low",
						Status:       "open",
						Category:     "Logging & Monitoring",
						ResourceType: "S3Bucket",
						ResourceName: bucket.Name,
						ResourceID:   bucket.Name,
						Standard:     "CIS AWS Foundations Benchmark",
						ControlMapping: []string{"CIS 2.1.5", "NIST AU-2"},
						Risk:         "Without access logging, unauthorized data access goes undetected.",
						Recommendation: "Enable S3 server access logging to a dedicated audit bucket.",
						Remediation:  "aws s3api put-bucket-logging --bucket " + bucket.Name + " --bucket-logging-status '{\"LoggingEnabled\":{\"TargetBucket\":\"<audit-bucket>\",\"TargetPrefix\":\"s3-access-logs/\" + bucket.Name + \"/\"}}'",
						Priority:     "P3",
						Timeline:     "This month",
					})
				}
			}
		}
	}

	// ── IAM ────────────────────────────────────────────────────────────────────
	if raw, ok := inv.Resources["iam"]; ok {
		b, _ := json.Marshal(raw)
		var iamData awsscanner.IAMData
		if err := json.Unmarshal(b, &iamData); err == nil {
			// Root MFA
			if !iamData.RootMFAEnabled {
				findings = append(findings, model.Finding{
					ID:           "aws-iam-root-no-mfa",
					Title:        "AWS root account has no MFA enabled",
					Severity:     "critical",
					Status:       "open",
					Category:     "Identity & Access Management",
					ResourceType: "IAMAccount",
					ResourceName: "root",
					Standard:     "CIS AWS Foundations Benchmark",
					ControlMapping: []string{"CIS 1.5", "NIST IA-2", "SOC2 CC6.1"},
					Risk:         "Root account with no MFA can be compromised with only a password, giving full AWS control.",
					BusinessImpact: "Complete account takeover, data destruction, financial damage from resource abuse.",
					Recommendation: "Enable MFA for the AWS root account immediately. Use a hardware MFA device.",
					Remediation:  "1. Sign in as root\n2. Go to IAM → Security credentials\n3. Under MFA, click Assign MFA device\n4. Use a virtual or hardware MFA device",
					Priority:     "P0",
					Timeline:     "Immediate",
				})
			}
			// Password policy
			if iamData.PasswordPolicy == nil {
				findings = append(findings, model.Finding{
					ID:           "aws-iam-no-password-policy",
					Title:        "No IAM account password policy configured",
					Severity:     "medium",
					Status:       "open",
					Category:     "Identity & Access Management",
					ResourceType: "IAMAccount",
					ResourceName: "account",
					Standard:     "CIS AWS Foundations Benchmark",
					ControlMapping: []string{"CIS 1.8-1.11", "NIST IA-5"},
					Risk:         "Users may set weak passwords, increasing risk of credential compromise.",
					Recommendation: "Configure a strong password policy: min 14 chars, uppercase, lowercase, numbers, symbols, max 90 days.",
					Remediation:  "aws iam update-account-password-policy --minimum-password-length 14 --require-symbols --require-numbers --require-uppercase-characters --require-lowercase-characters --max-password-age 90 --password-reuse-prevention 24",
					Priority:     "P2",
					Timeline:     "This week",
				})
			} else {
				pp := iamData.PasswordPolicy
				if pp.MinLength < 14 {
					findings = append(findings, model.Finding{
						ID:           "aws-iam-weak-password-policy",
						Title:        fmt.Sprintf("IAM password policy too weak (min length: %d, required: 14)", pp.MinLength),
						Severity:     "medium",
						Status:       "open",
						Category:     "Identity & Access Management",
						ResourceType: "IAMAccount",
						ResourceName: "password-policy",
						Standard:     "CIS AWS Foundations Benchmark",
						ControlMapping: []string{"CIS 1.8", "NIST IA-5"},
						Risk:         "Short passwords are easier to brute-force.",
						Recommendation: "Set minimum password length to 14 or more characters.",
						Remediation:  "aws iam update-account-password-policy --minimum-password-length 14",
						Priority:     "P2",
						Timeline:     "This week",
					})
				}
			}
			// Per-user checks
			for _, user := range iamData.Users {
				if !user.MFAEnabled && user.HasConsoleAccess {
					findings = append(findings, model.Finding{
						ID:           "aws-iam-user-no-mfa",
						Title:        "IAM user has console access but no MFA",
						Severity:     "high",
						Status:       "open",
						Category:     "Identity & Access Management",
						ResourceType: "IAMUser",
						ResourceName: user.UserName,
						ResourceID:   user.UserID,
						Standard:     "CIS AWS Foundations Benchmark",
						ControlMapping: []string{"CIS 1.10", "NIST IA-2", "SOC2 CC6.1"},
						Risk:         "Console access without MFA can be compromised with only a stolen password.",
						BusinessImpact: "Unauthorized access to AWS console and all its resources.",
						Recommendation: "Enforce MFA for all IAM users with console access.",
						Remediation:  "Enforce via IAM policy: Deny all actions unless aws:MultiFactorAuthPresent is true.\nOr use AWS Organizations SCP to enforce organization-wide.",
						Priority:     "P1",
						Timeline:     "Immediate",
					})
				}
				if user.AdminAccess && !user.MFAEnabled {
					findings = append(findings, model.Finding{
						ID:           "aws-iam-admin-no-mfa",
						Title:        "IAM admin user without MFA — critical risk",
						Severity:     "critical",
						Status:       "open",
						Category:     "Identity & Access Management",
						ResourceType: "IAMUser",
						ResourceName: user.UserName,
						ResourceID:   user.UserID,
						Standard:     "CIS AWS Foundations Benchmark",
						ControlMapping: []string{"CIS 1.6", "CIS 1.10", "NIST IA-2"},
						Risk:         "Admin user without MFA can fully control AWS account with just a password.",
						Recommendation: "Require MFA for all admin users immediately.",
						Priority:     "P0",
						Timeline:     "Immediate",
					})
				}
				if user.OldAccessKeys > 0 {
					findings = append(findings, model.Finding{
						ID:           "aws-iam-old-access-key",
						Title:        fmt.Sprintf("IAM user %s has access key older than 90 days", user.UserName),
						Severity:     "medium",
						Status:       "open",
						Category:     "Identity & Access Management",
						ResourceType: "IAMUser",
						ResourceName: user.UserName,
						ResourceID:   user.UserID,
						Standard:     "CIS AWS Foundations Benchmark",
						ControlMapping: []string{"CIS 1.14", "NIST IA-5"},
						Risk:         "Long-lived access keys increase the risk window if compromised.",
						Recommendation: "Rotate IAM access keys every 90 days.",
						Remediation:  "1. Create new access key: aws iam create-access-key --user-name " + user.UserName + "\n2. Update credentials in all systems\n3. Delete old key: aws iam delete-access-key --user-name " + user.UserName + " --access-key-id <OLD_KEY_ID>",
						Priority:     "P2",
						Timeline:     "This week",
					})
				}
				if user.InactiveAccessKeys > 0 {
					findings = append(findings, model.Finding{
						ID:           "aws-iam-inactive-key",
						Title:        fmt.Sprintf("IAM user %s has inactive access key (should be deleted)", user.UserName),
						Severity:     "low",
						Status:       "open",
						Category:     "Identity & Access Management",
						ResourceType: "IAMUser",
						ResourceName: user.UserName,
						ResourceID:   user.UserID,
						ControlMapping: []string{"CIS 1.12"},
						Risk:         "Inactive keys are unnecessary attack surface.",
						Recommendation: "Delete inactive access keys to reduce attack surface.",
						Remediation:  "aws iam delete-access-key --user-name " + user.UserName + " --access-key-id <KEY_ID>",
						Priority:     "P3",
						Timeline:     "This month",
					})
				}
			}
		}
	}

	// ── RDS Instances ──────────────────────────────────────────────────────────
	if raw, ok := inv.Resources["rds_instances"]; ok {
		b, _ := json.Marshal(raw)
		var instances []awsscanner.RDSInstance
		if err := json.Unmarshal(b, &instances); err == nil {
			for _, db := range instances {
				if db.PubliclyAccessible {
					findings = append(findings, model.Finding{
						ID:           "aws-rds-public",
						Title:        "RDS instance is publicly accessible",
						Severity:     "critical",
						Status:       "open",
						Category:     "Database Security",
						ResourceType: "RDSInstance",
						ResourceName: db.DBInstanceID,
						ResourceID:   db.DBInstanceID,
						Standard:     "CIS AWS Foundations Benchmark",
						ControlMapping: []string{"CIS 2.3.3", "NIST SC-7", "SOC2 CC6.6"},
						Risk:         "Publicly accessible database can be attacked directly from the internet.",
						BusinessImpact: "Database breach, data exfiltration, ransomware.",
						Recommendation: "Disable public accessibility and place RDS in private subnets. Access via VPN or bastion host.",
						Remediation:  "aws rds modify-db-instance --db-instance-identifier " + db.DBInstanceID + " --no-publicly-accessible --apply-immediately",
						Validation:   "aws rds describe-db-instances --db-instance-identifier " + db.DBInstanceID + " --query 'DBInstances[].PubliclyAccessible'",
						Priority:     "P0",
						Timeline:     "Immediate",
					})
				}
				if !db.StorageEncrypted {
					findings = append(findings, model.Finding{
						ID:           "aws-rds-no-encryption",
						Title:        "RDS instance storage not encrypted",
						Severity:     "high",
						Status:       "open",
						Category:     "Database Security",
						ResourceType: "RDSInstance",
						ResourceName: db.DBInstanceID,
						ResourceID:   db.DBInstanceID,
						Standard:     "CIS AWS Foundations Benchmark",
						ControlMapping: []string{"CIS 2.3.1", "NIST SC-28"},
						Risk:         "Unencrypted database storage exposes data if underlying storage is compromised.",
						Recommendation: "Enable encryption for RDS instances (requires migration to new instance).",
						Remediation:  "1. Create a snapshot\n2. Copy snapshot with encryption enabled\n3. Restore from encrypted snapshot\n(Note: encryption cannot be enabled on existing instances)",
						Priority:     "P1",
						Timeline:     "This week",
					})
				}
				if !db.MultiAZ {
					findings = append(findings, model.Finding{
						ID:           "aws-rds-no-multiaz",
						Title:        "RDS instance not configured for Multi-AZ",
						Severity:     "medium",
						Status:       "open",
						Category:     "High Availability",
						ResourceType: "RDSInstance",
						ResourceName: db.DBInstanceID,
						ResourceID:   db.DBInstanceID,
						ControlMapping: []string{"NIST CP-6"},
						Risk:         "Single-AZ deployment has no failover capability, causing downtime during outages.",
						Recommendation: "Enable Multi-AZ deployment for production databases.",
						Remediation:  "aws rds modify-db-instance --db-instance-identifier " + db.DBInstanceID + " --multi-az --apply-immediately",
						Priority:     "P2",
						Timeline:     "This week",
					})
				}
				if db.BackupRetentionDays < 7 {
					findings = append(findings, model.Finding{
						ID:           "aws-rds-short-backup",
						Title:        fmt.Sprintf("RDS backup retention too short (%d days, recommended: 7+)", db.BackupRetentionDays),
						Severity:     "medium",
						Status:       "open",
						Category:     "Data Protection",
						ResourceType: "RDSInstance",
						ResourceName: db.DBInstanceID,
						ResourceID:   db.DBInstanceID,
						ControlMapping: []string{"CIS 2.3.2", "NIST CP-9"},
						Risk:         "Short backup retention limits recovery options after a data loss event.",
						Recommendation: "Set backup retention to at least 7 days for all production databases.",
						Remediation:  "aws rds modify-db-instance --db-instance-identifier " + db.DBInstanceID + " --backup-retention-period 7 --apply-immediately",
						Priority:     "P2",
						Timeline:     "This week",
					})
				}
				if !db.DeletionProtection {
					findings = append(findings, model.Finding{
						ID:           "aws-rds-no-deletion-protection",
						Title:        "RDS instance deletion protection disabled",
						Severity:     "medium",
						Status:       "open",
						Category:     "Data Protection",
						ResourceType: "RDSInstance",
						ResourceName: db.DBInstanceID,
						ResourceID:   db.DBInstanceID,
						ControlMapping: []string{"NIST CP-9"},
						Risk:         "Database can be accidentally deleted without protection.",
						Recommendation: "Enable deletion protection for all production RDS instances.",
						Remediation:  "aws rds modify-db-instance --db-instance-identifier " + db.DBInstanceID + " --deletion-protection --apply-immediately",
						Priority:     "P2",
						Timeline:     "This week",
					})
				}
			}
		}
	}

	return findings
}

// AWSPositiveFindings generates positive findings for an AWS inventory.
func AWSPositiveFindings(inv model.Inventory) []model.PositiveFinding {
	var positives []model.PositiveFinding

	if raw, ok := inv.Resources["iam"]; ok {
		b, _ := json.Marshal(raw)
		var iamData awsscanner.IAMData
		if err := json.Unmarshal(b, &iamData); err == nil {
			if iamData.RootMFAEnabled {
				positives = append(positives, model.PositiveFinding{
					Area:     "IAM — Root Account MFA",
					Status:   "Enabled",
					Evidence: "AWS root account is protected with MFA",
				})
			}
		}
	}

	if raw, ok := inv.Resources["rds_instances"]; ok {
		b, _ := json.Marshal(raw)
		var instances []awsscanner.RDSInstance
		if err := json.Unmarshal(b, &instances); err == nil {
			encCount := 0
			for _, db := range instances {
				if db.StorageEncrypted {
					encCount++
				}
			}
			if encCount > 0 {
				positives = append(positives, model.PositiveFinding{
					Area:     "RDS — Storage Encryption",
					Status:   "Enabled",
					Evidence: fmt.Sprintf("%d of %d RDS instances have encrypted storage", encCount, len(instances)),
				})
			}
		}
	}

	return positives
}
