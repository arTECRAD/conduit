package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arTECRAD/conduit/server/internal/model"
	"github.com/arTECRAD/conduit/server/internal/repository"
	"github.com/arTECRAD/conduit/server/internal/service"
)

// fakeDeviceQuerier is an in-memory implementation of service.DeviceQuerier.
type fakeDeviceQuerier struct {
	devices map[uuid.UUID]repository.Device
}

func newFakeDeviceQuerier() *fakeDeviceQuerier {
	return &fakeDeviceQuerier{devices: make(map[uuid.UUID]repository.Device)}
}

func (f *fakeDeviceQuerier) CreateDevice(_ context.Context, arg repository.CreateDeviceParams) (repository.Device, error) {
	for _, d := range f.devices {
		if d.DeviceIdentifier == arg.DeviceIdentifier {
			return repository.Device{}, errDuplicate
		}
		if d.SetupCode == arg.SetupCode {
			return repository.Device{}, errDuplicate
		}
	}
	d := repository.Device{
		ID:               uuid.New(),
		DeviceIdentifier: arg.DeviceIdentifier,
		SetupCode:        arg.SetupCode,
		MqttUsername:     arg.MqttUsername,
		MqttPasswordHash: arg.MqttPasswordHash,
		Status:           repository.DeviceStatusPending,
		PollIntervalMs:   30000,
	}
	f.devices[d.ID] = d
	return d, nil
}

func (f *fakeDeviceQuerier) GetDeviceByID(_ context.Context, id uuid.UUID) (repository.Device, error) {
	d, ok := f.devices[id]
	if !ok {
		return repository.Device{}, errNotFound
	}
	return d, nil
}

func (f *fakeDeviceQuerier) GetDeviceByIdentifier(_ context.Context, identifier string) (repository.Device, error) {
	for _, d := range f.devices {
		if d.DeviceIdentifier == identifier {
			return d, nil
		}
	}
	return repository.Device{}, errNotFound
}

func (f *fakeDeviceQuerier) GetDeviceBySetupCode(_ context.Context, code string) (repository.Device, error) {
	for _, d := range f.devices {
		if d.SetupCode == code {
			return d, nil
		}
	}
	return repository.Device{}, errNotFound
}

func (f *fakeDeviceQuerier) ListDevicesByUserID(_ context.Context, userID *uuid.UUID) ([]repository.Device, error) {
	var result []repository.Device
	for _, d := range f.devices {
		if userID != nil && d.UserID != nil && *d.UserID == *userID {
			result = append(result, d)
		}
	}
	return result, nil
}

func (f *fakeDeviceQuerier) ClaimDevice(_ context.Context, arg repository.ClaimDeviceParams) (repository.Device, error) {
	d, ok := f.devices[arg.ID]
	if !ok {
		return repository.Device{}, errNotFound
	}
	if d.Status != repository.DeviceStatusPending {
		return repository.Device{}, errNotFound
	}
	d.UserID = arg.UserID
	d.Status = repository.DeviceStatusActive
	f.devices[arg.ID] = d
	return d, nil
}

func (f *fakeDeviceQuerier) UpdateDevice(_ context.Context, arg repository.UpdateDeviceParams) (repository.Device, error) {
	d, ok := f.devices[arg.ID]
	if !ok {
		return repository.Device{}, errNotFound
	}
	d.DisplayName = arg.DisplayName
	d.PollIntervalMs = arg.PollIntervalMs
	f.devices[arg.ID] = d
	return d, nil
}

func (f *fakeDeviceQuerier) DeleteDevice(_ context.Context, arg repository.DeleteDeviceParams) error {
	delete(f.devices, arg.ID)
	return nil
}

// noopCommander is a CommandPublisher that does nothing.
type noopCommander struct{}

func (noopCommander) Publish(_ context.Context, _ string, _ model.CommandPayload) error {
	return nil
}

func newTestDeviceService(q service.DeviceQuerier) *service.DeviceService {
	return service.NewDeviceService(q, noopCommander{}, nil, 4)
}

func TestDeviceService_Register(t *testing.T) {
	t.Run("happy path creates device and returns MQTT credentials", func(t *testing.T) {
		q := newFakeDeviceQuerier()
		svc := newTestDeviceService(q)

		resp, err := svc.Register(context.Background(), model.RegisterDeviceRequest{
			DeviceIdentifier: "aabbccdd",
			SetupCode:        "ABCD-1234",
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, resp.DeviceID)
		assert.Equal(t, "device_aabbccdd", resp.MQTTUsername)
		assert.NotEmpty(t, resp.MQTTPassword)
		assert.Equal(t, model.DeviceStatusPending, resp.Status)
	})

	t.Run("duplicate identifier returns ErrDeviceAlreadyRegistered", func(t *testing.T) {
		q := newFakeDeviceQuerier()
		svc := newTestDeviceService(q)

		_, err := svc.Register(context.Background(), model.RegisterDeviceRequest{
			DeviceIdentifier: "aabbccdd",
			SetupCode:        "ABCD-1234",
		})
		require.NoError(t, err)

		_, err = svc.Register(context.Background(), model.RegisterDeviceRequest{
			DeviceIdentifier: "aabbccdd",
			SetupCode:        "XXXX-9999",
		})
		assert.ErrorIs(t, err, service.ErrDeviceAlreadyRegistered)
	})
}

func TestDeviceService_Claim(t *testing.T) {
	t.Run("valid setup code claims device", func(t *testing.T) {
		q := newFakeDeviceQuerier()
		svc := newTestDeviceService(q)

		_, err := svc.Register(context.Background(), model.RegisterDeviceRequest{
			DeviceIdentifier: "aabbccdd",
			SetupCode:        "ABCD-1234",
		})
		require.NoError(t, err)

		userID := uuid.New()
		device, err := svc.Claim(context.Background(), userID, model.ClaimDeviceRequest{
			SetupCode: "ABCD-1234",
		})
		require.NoError(t, err)
		assert.Equal(t, model.DeviceStatusActive, device.Status)
		require.NotNil(t, device.UserID)
		assert.Equal(t, userID, *device.UserID)
	})

	t.Run("already-claimed device returns ErrDeviceAlreadyClaimed", func(t *testing.T) {
		q := newFakeDeviceQuerier()
		svc := newTestDeviceService(q)

		_, err := svc.Register(context.Background(), model.RegisterDeviceRequest{
			DeviceIdentifier: "aabbccdd",
			SetupCode:        "ABCD-1234",
		})
		require.NoError(t, err)

		userID := uuid.New()
		_, err = svc.Claim(context.Background(), userID, model.ClaimDeviceRequest{SetupCode: "ABCD-1234"})
		require.NoError(t, err)

		_, err = svc.Claim(context.Background(), uuid.New(), model.ClaimDeviceRequest{SetupCode: "ABCD-1234"})
		assert.ErrorIs(t, err, service.ErrDeviceAlreadyClaimed)
	})

	t.Run("unknown setup code returns ErrSetupCodeNotFound", func(t *testing.T) {
		q := newFakeDeviceQuerier()
		svc := newTestDeviceService(q)

		_, err := svc.Claim(context.Background(), uuid.New(), model.ClaimDeviceRequest{
			SetupCode: "XXXX-9999",
		})
		assert.ErrorIs(t, err, service.ErrSetupCodeNotFound)
	})
}
