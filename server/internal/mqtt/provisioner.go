package mqtt

import (
	"fmt"
	"log/slog"
	"os/exec"
)

// Provisioner manages MQTT user credentials in the Mosquitto password file.
type Provisioner struct {
	containerName string
	passwdPath    string
}

// NewProvisioner creates a new Provisioner.
// containerName is the Docker container name (e.g. "conduit-mosquitto").
// passwdPath is the path to the password file inside the container.
func NewProvisioner(containerName, passwdPath string) *Provisioner {
	return &Provisioner{
		containerName: containerName,
		passwdPath:    passwdPath,
	}
}

// AddUser adds or updates an MQTT user in the Mosquitto password file
// and sends SIGHUP to reload credentials.
func (p *Provisioner) AddUser(username, password string) error {
	// Add user via mosquitto_passwd inside the container
	cmd := exec.Command("docker", "exec", p.containerName,
		"mosquitto_passwd", "-b", p.passwdPath, username, password)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mosquitto_passwd: %w: %s", err, string(out))
	}

	// Send SIGHUP to reload the password file without restart
	reload := exec.Command("docker", "kill", "--signal=SIGHUP", p.containerName)
	if rOut, rErr := reload.CombinedOutput(); rErr != nil {
		slog.Warn("failed to reload Mosquitto after adding user", "err", rErr, "output", string(rOut))
	}

	slog.Info("MQTT user provisioned", "username", username)
	return nil
}
