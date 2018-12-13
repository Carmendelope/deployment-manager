package login_helper

import (
	"github.com/nalej/derrors"
	"google.golang.org/grpc/metadata"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
	"context"
	"os/user"
)

type Credentials struct {
	BasePath     string
	Token        string
	RefreshToken string
}

// NewCredentials creates a new Credentials structure.
func NewCredentials(basePath string, token string, refreshToken string) *Credentials {
	return &Credentials{basePath, token, refreshToken}
}

// Store the credentials in disk
func (c *Credentials) Store() derrors.Error {
	rPath := resolvePath(c.BasePath)
	_ = os.MkdirAll(rPath, 0700)
	tokenPath := filepath.Join(resolvePath(c.BasePath), TokenFileName)
	refreshTokenPath := filepath.Join(resolvePath(c.BasePath), RefreshTokenFileName)
	err := ioutil.WriteFile(tokenPath, []byte(c.Token), 0600)
	if err != nil {
		return derrors.AsError(err, "cannot write token file")
	}
	err = ioutil.WriteFile(refreshTokenPath, []byte(c.RefreshToken), 0600)
	if err != nil {
		return derrors.AsError(err, "cannot write refresh token file")
	}
	return nil
}

func (c *Credentials) GetContext(timeout ...time.Duration) (context.Context, context.CancelFunc) {
	md := metadata.New(map[string]string{AuthHeader: c.Token})
	//log.Debug().Interface("md", md).Msg("metadata has been created")
	if len(timeout) == 0 {
		baseContext, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
		return metadata.NewOutgoingContext(baseContext, md), cancel
	}
	baseContext, cancel := context.WithTimeout(context.Background(), timeout[0])
	return metadata.NewOutgoingContext(baseContext, md), cancel
}

func resolvePath(path string) string {
	if strings.HasPrefix(path, "~") {
		usr, _ := user.Current()
		return strings.Replace(path, "~", usr.HomeDir, 1)
	}
	if strings.HasPrefix(path, ".") {
		abs, _ := filepath.Abs("./")
		return strings.Replace(path, ".", abs, 1)
	}
	return path
}