package remote

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/grafana/grafana/pkg/infra/log"
	apimodels "github.com/grafana/grafana/pkg/services/ngalert/api/tooling/definitions"
	"github.com/grafana/grafana/pkg/services/ngalert/models"
	"github.com/grafana/grafana/pkg/services/ngalert/notifier"
)

//go:generate mockery --name remoteAlertmanager --structname RemoteAlertmanagerMock --with-expecter --output mock --outpkg alertmanager_mock
type remoteAlertmanager interface {
	notifier.Alertmanager
	CompareAndSendConfiguration(context.Context, *models.AlertConfiguration) error
	CompareAndSendState(context.Context) error
}

type RemoteSecondaryForkedAlertmanager struct {
	log log.Logger

	internal notifier.Alertmanager
	remote   remoteAlertmanager

	lastSync     time.Time
	syncInterval time.Duration
}

type RemoteSecondaryConfig struct {
	// SyncInterval determines how often we should attempt to synchronize
	// state and configuration on the external Alertmanager.
	SyncInterval time.Duration
	Logger       log.Logger
}

func (c *RemoteSecondaryConfig) Validate() error {
	if c.Logger == nil {
		return fmt.Errorf("logger cannot be nil")
	}
	return nil
}

func NewRemoteSecondaryForkedAlertmanager(cfg RemoteSecondaryConfig, internal notifier.Alertmanager, remote remoteAlertmanager) (*RemoteSecondaryForkedAlertmanager, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &RemoteSecondaryForkedAlertmanager{
		log:          cfg.Logger,
		internal:     internal,
		remote:       remote,
		syncInterval: cfg.SyncInterval,
	}, nil
}

// ApplyConfig will only log errors for the remote Alertmanager and ensure we delegate the call to the internal Alertmanager.
// We don't care about errors in the remote Alertmanager in remote secondary mode.
func (fam *RemoteSecondaryForkedAlertmanager) ApplyConfig(ctx context.Context, config *models.AlertConfiguration) error {
	var wg sync.WaitGroup
	wg.Add(1)
	// Figure out if we need to sync the external Alertmanager in another goroutine.
	go func() {
		defer wg.Done()
		// If the Alertmanager has not been marked as "ready" yet, delegate the call to the remote Alertmanager.
		// This will perform a readiness check and sync the Alertmanagers.
		if !fam.remote.Ready() {
			if err := fam.remote.ApplyConfig(ctx, config); err != nil {
				fam.log.Error("Error applying config to the remote Alertmanager", "err", err)
				return
			}
			fam.lastSync = time.Now()
			return
		}

		// If the Alertmanager was marked as ready but the sync interval has elapsed, sync the Alertmanagers.
		if time.Since(fam.lastSync) >= fam.syncInterval {
			fam.log.Debug("Syncing configuration and state with the remote Alertmanager", "lastSync", fam.lastSync)
			cfgErr := fam.remote.CompareAndSendConfiguration(ctx, config)
			if cfgErr != nil {
				fam.log.Error("Unable to upload the configuration to the remote Alertmanager", "err", cfgErr)
			}

			stateErr := fam.remote.CompareAndSendState(ctx)
			if stateErr != nil {
				fam.log.Error("Unable to upload the state to the remote Alertmanager", "err", stateErr)
			}
			fam.log.Debug("Finished syncing configuration and state with the remote Alertmanager")

			if cfgErr == nil && stateErr == nil {
				fam.lastSync = time.Now()
			}
		}
	}()

	// Call ApplyConfig on the internal Alertmanager - we only care about errors for this one.
	err := fam.internal.ApplyConfig(ctx, config)
	wg.Wait()
	return err
}

// SaveAndApplyConfig is only called on the internal Alertmanager when running in remote secondary mode.
func (fam *RemoteSecondaryForkedAlertmanager) SaveAndApplyConfig(ctx context.Context, config *apimodels.PostableUserConfig) error {
	return fam.internal.SaveAndApplyConfig(ctx, config)
}

// SaveAndApplyDefaultConfig is only called on the internal Alertmanager when running in remote secondary mode.
func (fam *RemoteSecondaryForkedAlertmanager) SaveAndApplyDefaultConfig(ctx context.Context) error {
	return fam.internal.SaveAndApplyDefaultConfig(ctx)
}

func (fam *RemoteSecondaryForkedAlertmanager) GetStatus() apimodels.GettableStatus {
	return fam.internal.GetStatus()
}

func (fam *RemoteSecondaryForkedAlertmanager) CreateSilence(ctx context.Context, silence *apimodels.PostableSilence) (string, error) {
	return fam.internal.CreateSilence(ctx, silence)
}

func (fam *RemoteSecondaryForkedAlertmanager) DeleteSilence(ctx context.Context, id string) error {
	return fam.internal.DeleteSilence(ctx, id)
}

func (fam *RemoteSecondaryForkedAlertmanager) GetSilence(ctx context.Context, id string) (apimodels.GettableSilence, error) {
	return fam.internal.GetSilence(ctx, id)
}

func (fam *RemoteSecondaryForkedAlertmanager) ListSilences(ctx context.Context, filter []string) (apimodels.GettableSilences, error) {
	return fam.internal.ListSilences(ctx, filter)
}

func (fam *RemoteSecondaryForkedAlertmanager) GetAlerts(ctx context.Context, active, silenced, inhibited bool, filter []string, receiver string) (apimodels.GettableAlerts, error) {
	return fam.internal.GetAlerts(ctx, active, silenced, inhibited, filter, receiver)
}

func (fam *RemoteSecondaryForkedAlertmanager) GetAlertGroups(ctx context.Context, active, silenced, inhibited bool, filter []string, receiver string) (apimodels.AlertGroups, error) {
	return fam.internal.GetAlertGroups(ctx, active, silenced, inhibited, filter, receiver)
}

func (fam *RemoteSecondaryForkedAlertmanager) PutAlerts(ctx context.Context, alerts apimodels.PostableAlerts) error {
	return fam.internal.PutAlerts(ctx, alerts)
}

func (fam *RemoteSecondaryForkedAlertmanager) GetReceivers(ctx context.Context) ([]apimodels.Receiver, error) {
	return fam.internal.GetReceivers(ctx)
}

func (fam *RemoteSecondaryForkedAlertmanager) TestReceivers(ctx context.Context, c apimodels.TestReceiversConfigBodyParams) (*notifier.TestReceiversResult, error) {
	return fam.internal.TestReceivers(ctx, c)
}

func (fam *RemoteSecondaryForkedAlertmanager) TestTemplate(ctx context.Context, c apimodels.TestTemplatesConfigBodyParams) (*notifier.TestTemplatesResults, error) {
	return fam.internal.TestTemplate(ctx, c)
}

func (fam *RemoteSecondaryForkedAlertmanager) CleanUp() {
	// No cleanup to do in the remote Alertmanager.
	fam.internal.CleanUp()
}

func (fam *RemoteSecondaryForkedAlertmanager) StopAndWait() {
	fam.internal.StopAndWait()
	fam.remote.StopAndWait()
	// TODO: send config and state on shutdown.
}

func (fam *RemoteSecondaryForkedAlertmanager) Ready() bool {
	// We only care about the internal Alertmanager being ready.
	return fam.internal.Ready()
}
