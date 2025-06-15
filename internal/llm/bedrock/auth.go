package bedrock

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// AuthConfig represents authentication configuration options
type AuthConfig struct {
	// Method specifies the authentication method
	Method AuthMethod `yaml:"method"`

	// Static credentials
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	SessionToken    string `yaml:"session_token"`

	// Profile-based auth
	Profile string `yaml:"profile"`

	// Region
	Region string `yaml:"region"`

	// Role ARN for assume role
	RoleARN         string `yaml:"role_arn"`
	RoleSessionName string `yaml:"role_session_name"`
	ExternalID      string `yaml:"external_id"`
}

// AuthMethod represents different authentication methods
type AuthMethod string

const (
	AuthMethodDefault     AuthMethod = "default"     // AWS SDK default chain
	AuthMethodStatic      AuthMethod = "static"      // Access key/secret
	AuthMethodProfile     AuthMethod = "profile"     // AWS profile
	AuthMethodRole        AuthMethod = "assume_role" // Assume role
	AuthMethodEnvironment AuthMethod = "environment" // Environment variables only
)

// LoadAWSConfig loads AWS configuration using the specified authentication method
func LoadAWSConfig(ctx context.Context, authCfg *AuthConfig) (aws.Config, error) {
	var configOptions []func(*config.LoadOptions) error

	// Set region
	if authCfg.Region != "" {
		configOptions = append(configOptions, config.WithRegion(authCfg.Region))
	}

	switch authCfg.Method {
	case AuthMethodStatic:
		if authCfg.AccessKeyID == "" || authCfg.SecretAccessKey == "" {
			return aws.Config{}, fmt.Errorf("access_key_id and secret_access_key required for static authentication")
		}

		creds := credentials.NewStaticCredentialsProvider(
			authCfg.AccessKeyID,
			authCfg.SecretAccessKey,
			authCfg.SessionToken,
		)
		configOptions = append(configOptions, config.WithCredentialsProvider(creds))

	case AuthMethodProfile:
		if authCfg.Profile == "" {
			return aws.Config{}, fmt.Errorf("profile name required for profile authentication")
		}
		configOptions = append(configOptions, config.WithSharedConfigProfile(authCfg.Profile))

	case AuthMethodEnvironment:
		// Force environment variables only
		creds := credentials.NewStaticCredentialsProvider("", "", "")
		configOptions = append(configOptions, config.WithCredentialsProvider(creds))

	case AuthMethodRole:
		if authCfg.RoleARN == "" {
			return aws.Config{}, fmt.Errorf("role_arn required for assume role authentication")
		}

		// First load config to get base credentials
		baseCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(authCfg.Region))
		if err != nil {
			return aws.Config{}, fmt.Errorf("failed to load base config for role assumption: %w", err)
		}

		// Create STS client for assume role
		stsClient := sts.NewFromConfig(baseCfg)

		// Set up assume role provider
		roleProvider := &AssumeRoleProvider{
			client:          stsClient,
			roleARN:         authCfg.RoleARN,
			roleSessionName: authCfg.RoleSessionName,
			externalID:      authCfg.ExternalID,
		}

		configOptions = append(configOptions, config.WithCredentialsProvider(roleProvider))

	case AuthMethodDefault:
		fallthrough
	default:
		// Use default AWS credential chain
		// This includes: env vars, shared credentials, IAM roles, etc.
	}

	// Load the configuration
	awsCfg, err := config.LoadDefaultConfig(ctx, configOptions...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return awsCfg, nil
}

// ValidateCredentials validates that the credentials work
func ValidateCredentials(ctx context.Context, cfg aws.Config) error {
	stsClient := sts.NewFromConfig(cfg)

	// Try to get caller identity
	_, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("credential validation failed: %w", err)
	}

	return nil
}

// AssumeRoleProvider implements credentials.Provider for assume role
type AssumeRoleProvider struct {
	client          *sts.Client
	roleARN         string
	roleSessionName string
	externalID      string
}

// Retrieve implements the credentials.Provider interface
func (p *AssumeRoleProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	sessionName := p.roleSessionName
	if sessionName == "" {
		sessionName = "bazinga-session"
	}

	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(p.roleARN),
		RoleSessionName: aws.String(sessionName),
	}

	if p.externalID != "" {
		input.ExternalId = aws.String(p.externalID)
	}

	result, err := p.client.AssumeRole(ctx, input)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to assume role: %w", err)
	}

	creds := result.Credentials
	return aws.Credentials{
		AccessKeyID:     *creds.AccessKeyId,
		SecretAccessKey: *creds.SecretAccessKey,
		SessionToken:    *creds.SessionToken,
		Expires:         *creds.Expiration,
	}, nil
}
