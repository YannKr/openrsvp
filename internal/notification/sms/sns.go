package sms

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/openrsvp/openrsvp/internal/notification"
)

// SNSProvider sends SMS messages via Amazon SNS.
type SNSProvider struct {
	snsClient *sns.Client
}

// NewSNSProvider creates a new SNSProvider with the given AWS region and
// explicit credentials.
func NewSNSProvider(region, accessKeyID, secretAccessKey string) (*SNSProvider, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("sns load aws config: %w", err)
	}

	return &SNSProvider{
		snsClient: sns.NewFromConfig(cfg),
	}, nil
}

// Name returns the provider identifier.
func (p *SNSProvider) Name() string {
	return "sns"
}

// Channel returns which channel this provider serves.
func (p *SNSProvider) Channel() notification.Channel {
	return notification.ChannelSMS
}

// Send delivers a single SMS via Amazon SNS Publish.
func (p *SNSProvider) Send(ctx context.Context, msg *notification.Message) error {
	_, err := p.snsClient.Publish(ctx, &sns.PublishInput{
		PhoneNumber: aws.String(msg.To),
		Message:     aws.String(msg.Body),
	})
	if err != nil {
		return fmt.Errorf("sns publish: %w", err)
	}
	return nil
}

// SendBatch delivers multiple SMS messages by iterating and sending each
// one individually.
func (p *SNSProvider) SendBatch(ctx context.Context, msgs []*notification.Message) []error {
	errs := make([]error, len(msgs))
	for i, msg := range msgs {
		errs[i] = p.Send(ctx, msg)
	}
	return errs
}

// HealthCheck verifies the SNS credentials by fetching SMS attributes.
func (p *SNSProvider) HealthCheck(ctx context.Context) error {
	_, err := p.snsClient.GetSMSAttributes(ctx, &sns.GetSMSAttributesInput{})
	if err != nil {
		return fmt.Errorf("sns health check: %w", err)
	}
	return nil
}
