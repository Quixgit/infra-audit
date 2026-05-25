// Package aws provides an AWS infrastructure security scanner.
package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"infra-audit/internal/model"
)

// ScanOptions holds AWS credentials and scan configuration.
type ScanOptions struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
}

// Scan performs a security scan of AWS resources.
func Scan(clientName string, opts ScanOptions) (model.Inventory, error) {
	inv := model.Inventory{
		Client:      clientName,
		Provider:    "AWS",
		CollectedAt: time.Now().UTC().Format(time.RFC3339),
		Resources:   make(map[string]interface{}),
	}

	if opts.Region == "" {
		opts.Region = "us-east-1"
	}

	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(opts.Region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(opts.AccessKeyID, opts.SecretAccessKey, ""),
		),
	)
	if err != nil {
		return inv, fmt.Errorf("aws config: %w", err)
	}

	inv.Scope = []string{fmt.Sprintf("AWS region: %s", opts.Region)}

	// ── EC2 Instances ──────────────────────────────────────────────────────────
	ec2Client := ec2.NewFromConfig(cfg)
	instances, ec2Err := scanEC2(ctx, ec2Client, &inv)
	if ec2Err != nil {
		inv.Errors = append(inv.Errors, "EC2: "+ec2Err.Error())
	}
	inv.Resources["ec2_instances"] = instances

	// ── Security Groups ────────────────────────────────────────────────────────
	sgs, sgErr := scanSecurityGroups(ctx, ec2Client, &inv)
	if sgErr != nil {
		inv.Errors = append(inv.Errors, "SecurityGroups: "+sgErr.Error())
	}
	inv.Resources["security_groups"] = sgs

	// ── VPCs ───────────────────────────────────────────────────────────────────
	vpcs, vpcErr := scanVPCs(ctx, ec2Client, &inv)
	if vpcErr != nil {
		inv.Errors = append(inv.Errors, "VPCs: "+vpcErr.Error())
	}
	inv.Resources["vpcs"] = vpcs

	// ── S3 Buckets ─────────────────────────────────────────────────────────────
	s3Client := s3.NewFromConfig(cfg)
	buckets, s3Err := scanS3(ctx, s3Client, &inv)
	if s3Err != nil {
		inv.Errors = append(inv.Errors, "S3: "+s3Err.Error())
	}
	inv.Resources["s3_buckets"] = buckets

	// ── IAM ────────────────────────────────────────────────────────────────────
	iamClient := iam.NewFromConfig(cfg)
	iamData, iamErr := scanIAM(ctx, iamClient, &inv)
	if iamErr != nil {
		inv.Errors = append(inv.Errors, "IAM: "+iamErr.Error())
	}
	inv.Resources["iam"] = iamData

	// ── RDS Instances ──────────────────────────────────────────────────────────
	rdsClient := rds.NewFromConfig(cfg)
	rdsInstances, rdsErr := scanRDS(ctx, rdsClient, &inv)
	if rdsErr != nil {
		inv.Errors = append(inv.Errors, "RDS: "+rdsErr.Error())
	}
	inv.Resources["rds_instances"] = rdsInstances

	return inv, nil
}

// ── EC2 ────────────────────────────────────────────────────────────────────────

type EC2Instance struct {
	InstanceID         string   `json:"instance_id"`
	Name               string   `json:"name"`
	State              string   `json:"state"`
	InstanceType       string   `json:"instance_type"`
	PublicIP           string   `json:"public_ip"`
	PrivateIP          string   `json:"private_ip"`
	PublicDNS          string   `json:"public_dns"`
	SecurityGroupIDs   []string `json:"security_group_ids"`
	KeyName            string   `json:"key_name"`
	IMDSv2Required     bool     `json:"imdsv2_required"`
	EBSEncrypted       bool     `json:"ebs_encrypted"`
	MonitoringEnabled  bool     `json:"monitoring_enabled"`
	Region             string   `json:"region"`
}

func scanEC2(ctx context.Context, client *ec2.Client, inv *model.Inventory) ([]EC2Instance, error) {
	var out []EC2Instance
	paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return out, err
		}
		for _, r := range page.Reservations {
			for _, i := range r.Instances {
				if i.State != nil && i.State.Name == "terminated" {
					continue
				}
				inst := EC2Instance{
					InstanceID:  aws.ToString(i.InstanceId),
					State:       string(i.State.Name),
					InstanceType: string(i.InstanceType),
					PublicIP:    aws.ToString(i.PublicIpAddress),
					PrivateIP:   aws.ToString(i.PrivateIpAddress),
					PublicDNS:   aws.ToString(i.PublicDnsName),
					KeyName:     aws.ToString(i.KeyName),
					Region:      inv.Scope[0],
				}
				// name tag
				for _, tag := range i.Tags {
					if aws.ToString(tag.Key) == "Name" {
						inst.Name = aws.ToString(tag.Value)
					}
				}
				// security groups
				for _, sg := range i.SecurityGroups {
					inst.SecurityGroupIDs = append(inst.SecurityGroupIDs, aws.ToString(sg.GroupId))
				}
				// IMDSv2
				if i.MetadataOptions != nil {
					inst.IMDSv2Required = i.MetadataOptions.HttpTokens == ec2types.HttpTokensStateRequired
				}
				// EBS encryption
				for _, bdm := range i.BlockDeviceMappings {
					if bdm.Ebs != nil && bdm.Ebs.VolumeId != nil {
						// Check if encrypted — simplified; full check requires DescribeVolumes
						inst.EBSEncrypted = false
					}
				}
				// Monitoring
				if i.Monitoring != nil {
					inst.MonitoringEnabled = i.Monitoring.State == ec2types.MonitoringStateEnabled
				}
				out = append(out, inst)
			}
		}
	}
	return out, nil
}

// ── Security Groups ────────────────────────────────────────────────────────────

type SecurityGroup struct {
	GroupID     string          `json:"group_id"`
	GroupName   string          `json:"group_name"`
	Description string          `json:"description"`
	VpcID       string          `json:"vpc_id"`
	IngressRules []SGRule       `json:"ingress_rules"`
	EgressRules  []SGRule       `json:"egress_rules"`
	OpenToWorld  bool           `json:"open_to_world"`
	CriticalPorts []int         `json:"critical_ports_exposed"`
}

type SGRule struct {
	Protocol  string   `json:"protocol"`
	FromPort  int32    `json:"from_port"`
	ToPort    int32    `json:"to_port"`
	CIDRs     []string `json:"cidrs"`
	IPv6CIDRs []string `json:"ipv6_cidrs"`
}

var criticalPorts = []int32{22, 3389, 3306, 5432, 27017, 6379, 9200, 8080, 8443}

func scanSecurityGroups(ctx context.Context, client *ec2.Client, inv *model.Inventory) ([]SecurityGroup, error) {
	var out []SecurityGroup
	paginator := ec2.NewDescribeSecurityGroupsPaginator(client, &ec2.DescribeSecurityGroupsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return out, err
		}
		for _, sg := range page.SecurityGroups {
			s := SecurityGroup{
				GroupID:     aws.ToString(sg.GroupId),
				GroupName:   aws.ToString(sg.GroupName),
				Description: aws.ToString(sg.Description),
				VpcID:       aws.ToString(sg.VpcId),
			}
			for _, r := range sg.IpPermissions {
				rule := toSGRule(r)
				s.IngressRules = append(s.IngressRules, rule)
				// Check open to world
				for _, cidr := range rule.CIDRs {
					if cidr == "0.0.0.0/0" {
						s.OpenToWorld = true
						// Check critical ports
						for _, cp := range criticalPorts {
							if (r.FromPort != nil && r.ToPort != nil && *r.FromPort <= cp && cp <= *r.ToPort) ||
								(r.FromPort == nil && r.ToPort == nil) {
								s.CriticalPorts = appendUnique(s.CriticalPorts, int(cp))
							}
						}
					}
				}
				for _, cidr := range rule.IPv6CIDRs {
					if cidr == "::/0" {
						s.OpenToWorld = true
					}
				}
			}
			for _, r := range sg.IpPermissionsEgress {
				s.EgressRules = append(s.EgressRules, toSGRule(r))
			}
			out = append(out, s)
		}
	}
	return out, nil
}

func toSGRule(r ec2types.IpPermission) SGRule {
	rule := SGRule{
		Protocol: aws.ToString(r.IpProtocol),
	}
	if r.FromPort != nil {
		rule.FromPort = *r.FromPort
	}
	if r.ToPort != nil {
		rule.ToPort = *r.ToPort
	}
	for _, c := range r.IpRanges {
		rule.CIDRs = append(rule.CIDRs, aws.ToString(c.CidrIp))
	}
	for _, c := range r.Ipv6Ranges {
		rule.IPv6CIDRs = append(rule.IPv6CIDRs, aws.ToString(c.CidrIpv6))
	}
	return rule
}

func appendUnique(s []int, v int) []int {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

// ── VPCs ───────────────────────────────────────────────────────────────────────

type VPC struct {
	VpcID      string `json:"vpc_id"`
	Name       string `json:"name"`
	CidrBlock  string `json:"cidr_block"`
	IsDefault  bool   `json:"is_default"`
	State      string `json:"state"`
	FlowLogs   bool   `json:"flow_logs_enabled"`
}

func scanVPCs(ctx context.Context, client *ec2.Client, inv *model.Inventory) ([]VPC, error) {
	var out []VPC
	result, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return out, err
	}
	// Get flow logs
	flowLogsResult, _ := client.DescribeFlowLogs(ctx, &ec2.DescribeFlowLogsInput{})
	flowLogVPCs := map[string]bool{}
	if flowLogsResult != nil {
		for _, fl := range flowLogsResult.FlowLogs {
			if fl.ResourceId != nil {
				flowLogVPCs[*fl.ResourceId] = true
			}
		}
	}
	for _, v := range result.Vpcs {
		vpc := VPC{
			VpcID:     aws.ToString(v.VpcId),
			CidrBlock: aws.ToString(v.CidrBlock),
			IsDefault: aws.ToBool(v.IsDefault),
			State:     string(v.State),
			FlowLogs:  flowLogVPCs[aws.ToString(v.VpcId)],
		}
		for _, tag := range v.Tags {
			if aws.ToString(tag.Key) == "Name" {
				vpc.Name = aws.ToString(tag.Value)
			}
		}
		out = append(out, vpc)
	}
	return out, nil
}

// ── S3 ─────────────────────────────────────────────────────────────────────────

type S3Bucket struct {
	Name              string `json:"name"`
	Region            string `json:"region"`
	PublicACL         bool   `json:"public_acl"`
	PublicPolicy      bool   `json:"public_policy"`
	BlockPublicAccess bool   `json:"block_public_access"`
	VersioningEnabled bool   `json:"versioning_enabled"`
	EncryptionEnabled bool   `json:"encryption_enabled"`
	MFADeleteEnabled  bool   `json:"mfa_delete_enabled"`
	LoggingEnabled    bool   `json:"logging_enabled"`
}

func scanS3(ctx context.Context, client *s3.Client, inv *model.Inventory) ([]S3Bucket, error) {
	var out []S3Bucket
	result, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return out, err
	}
	for _, b := range result.Buckets {
		name := aws.ToString(b.Name)
		bucket := S3Bucket{Name: name}

		// Region
		locResult, err := client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: aws.String(name)})
		if err == nil && locResult.LocationConstraint != "" {
			bucket.Region = string(locResult.LocationConstraint)
		} else {
			bucket.Region = "us-east-1"
		}

		// Block public access
		bpa, err := client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: aws.String(name)})
		if err == nil && bpa.PublicAccessBlockConfiguration != nil {
			c := bpa.PublicAccessBlockConfiguration
			bucket.BlockPublicAccess = aws.ToBool(c.BlockPublicAcls) &&
				aws.ToBool(c.BlockPublicPolicy) &&
				aws.ToBool(c.IgnorePublicAcls) &&
				aws.ToBool(c.RestrictPublicBuckets)
		}

		// Versioning
		ver, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(name)})
		if err == nil {
			bucket.VersioningEnabled = ver.Status == "Enabled"
			bucket.MFADeleteEnabled = ver.MFADelete == "Enabled"
		}

		// Encryption
		enc, err := client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{Bucket: aws.String(name)})
		if err == nil && enc.ServerSideEncryptionConfiguration != nil {
			bucket.EncryptionEnabled = len(enc.ServerSideEncryptionConfiguration.Rules) > 0
		}

		// Logging
		log, err := client.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{Bucket: aws.String(name)})
		if err == nil && log.LoggingEnabled != nil {
			bucket.LoggingEnabled = log.LoggingEnabled.TargetBucket != nil
		}

		out = append(out, bucket)
	}
	return out, nil
}

// ── IAM ────────────────────────────────────────────────────────────────────────

type IAMData struct {
	Users          []IAMUser   `json:"users"`
	PasswordPolicy *IAMPassPolicy `json:"password_policy"`
	RootMFAEnabled bool        `json:"root_mfa_enabled"`
	AccountID      string      `json:"account_id"`
}

type IAMUser struct {
	UserName           string   `json:"username"`
	UserID             string   `json:"user_id"`
	ARN                string   `json:"arn"`
	MFAEnabled         bool     `json:"mfa_enabled"`
	HasConsoleAccess   bool     `json:"has_console_access"`
	AccessKeyCount     int      `json:"access_key_count"`
	InactiveAccessKeys int      `json:"inactive_access_keys"`
	OldAccessKeys      int      `json:"old_access_keys_90d"`
	AdminAccess        bool     `json:"admin_access"`
	LastLoginDays      int      `json:"last_login_days"`
	Groups             []string `json:"groups"`
}

type IAMPassPolicy struct {
	MinLength             int  `json:"min_length"`
	RequireUppercase      bool `json:"require_uppercase"`
	RequireLowercase      bool `json:"require_lowercase"`
	RequireNumbers        bool `json:"require_numbers"`
	RequireSymbols        bool `json:"require_symbols"`
	MaxPasswordAge        int  `json:"max_password_age"`
	PasswordReuse         int  `json:"password_reuse_prevention"`
	HardExpiry            bool `json:"hard_expiry"`
}

func scanIAM(ctx context.Context, client *iam.Client, inv *model.Inventory) (IAMData, error) {
	data := IAMData{}

	// Get account summary
	summary, err := client.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err == nil {
		data.RootMFAEnabled = summary.SummaryMap["AccountMFAEnabled"] == 1
	}

	// Password policy
	pp, err := client.GetAccountPasswordPolicy(ctx, &iam.GetAccountPasswordPolicyInput{})
	if err == nil && pp.PasswordPolicy != nil {
		pol := pp.PasswordPolicy
		data.PasswordPolicy = &IAMPassPolicy{
			MinLength:        int(aws.ToInt32(pol.MinimumPasswordLength)),
			RequireUppercase: pol.RequireUppercaseCharacters,
			RequireLowercase: pol.RequireLowercaseCharacters,
			RequireNumbers:   pol.RequireNumbers,
			RequireSymbols:   pol.RequireSymbols,
			MaxPasswordAge:   int(aws.ToInt32(pol.MaxPasswordAge)),
			PasswordReuse:    int(aws.ToInt32(pol.PasswordReusePrevention)),
			HardExpiry:       aws.ToBool(pol.HardExpiry),
		}
	}

	// List users
	userPaginator := iam.NewListUsersPaginator(client, &iam.ListUsersInput{})
	for userPaginator.HasMorePages() {
		page, err := userPaginator.NextPage(ctx)
		if err != nil {
			return data, err
		}
		for _, u := range page.Users {
			user := IAMUser{
				UserName: aws.ToString(u.UserName),
				UserID:   aws.ToString(u.UserId),
				ARN:      aws.ToString(u.Arn),
			}
			// Last login
			if u.PasswordLastUsed != nil {
				days := int(time.Since(*u.PasswordLastUsed).Hours() / 24)
				user.LastLoginDays = days
				user.HasConsoleAccess = true
			}
			// MFA devices
			mfa, err := client.ListMFADevices(ctx, &iam.ListMFADevicesInput{UserName: u.UserName})
			if err == nil {
				user.MFAEnabled = len(mfa.MFADevices) > 0
			}
			// Access keys
			keys, err := client.ListAccessKeys(ctx, &iam.ListAccessKeysInput{UserName: u.UserName})
			if err == nil {
				user.AccessKeyCount = len(keys.AccessKeyMetadata)
				for _, k := range keys.AccessKeyMetadata {
					if k.Status == "Inactive" {
						user.InactiveAccessKeys++
					}
					if k.CreateDate != nil && time.Since(*k.CreateDate).Hours() > 90*24 {
						user.OldAccessKeys++
					}
				}
			}
			// Admin check (via attached policies)
			attached, err := client.ListAttachedUserPolicies(ctx, &iam.ListAttachedUserPoliciesInput{UserName: u.UserName})
			if err == nil {
				for _, p := range attached.AttachedPolicies {
					if strings.Contains(aws.ToString(p.PolicyName), "AdministratorAccess") {
						user.AdminAccess = true
					}
				}
			}
			// Groups
			groups, err := client.ListGroupsForUser(ctx, &iam.ListGroupsForUserInput{UserName: u.UserName})
			if err == nil {
				for _, g := range groups.Groups {
					user.Groups = append(user.Groups, aws.ToString(g.GroupName))
				}
			}
			data.Users = append(data.Users, user)
		}
	}
	return data, nil
}

// ── RDS ────────────────────────────────────────────────────────────────────────

type RDSInstance struct {
	DBInstanceID         string `json:"db_instance_id"`
	Engine               string `json:"engine"`
	EngineVersion        string `json:"engine_version"`
	Status               string `json:"status"`
	PubliclyAccessible   bool   `json:"publicly_accessible"`
	StorageEncrypted     bool   `json:"storage_encrypted"`
	MultiAZ              bool   `json:"multi_az"`
	BackupRetentionDays  int    `json:"backup_retention_days"`
	AutoMinorVersionUpgrade bool `json:"auto_minor_version_upgrade"`
	DeletionProtection   bool   `json:"deletion_protection"`
	DBInstanceClass      string `json:"db_instance_class"`
	Endpoint             string `json:"endpoint"`
}

func scanRDS(ctx context.Context, client *rds.Client, inv *model.Inventory) ([]RDSInstance, error) {
	var out []RDSInstance
	paginator := rds.NewDescribeDBInstancesPaginator(client, &rds.DescribeDBInstancesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return out, err
		}
		for _, db := range page.DBInstances {
			inst := RDSInstance{
				DBInstanceID:          aws.ToString(db.DBInstanceIdentifier),
				Engine:                aws.ToString(db.Engine),
				EngineVersion:         aws.ToString(db.EngineVersion),
				Status:                aws.ToString(db.DBInstanceStatus),
				PubliclyAccessible:    aws.ToBool(db.PubliclyAccessible),
				StorageEncrypted:      aws.ToBool(db.StorageEncrypted),
				MultiAZ:               aws.ToBool(db.MultiAZ),
				BackupRetentionDays:   int(aws.ToInt32(db.BackupRetentionPeriod)),
				AutoMinorVersionUpgrade: aws.ToBool(db.AutoMinorVersionUpgrade),
				DeletionProtection:    aws.ToBool(db.DeletionProtection),
				DBInstanceClass:       aws.ToString(db.DBInstanceClass),
			}
			if db.Endpoint != nil {
				inst.Endpoint = aws.ToString(db.Endpoint.Address)
			}
			out = append(out, inst)
		}
	}
	return out, nil
}
