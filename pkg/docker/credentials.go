package docker

// Credentials holds the information shared between docker and the credentials store.
type Credentials struct {
	ServerURL string
	Username  string
	Secret    string
}

func (c *Credentials) AuthToken() string {
	if c.Username != "" {
		return c.Username + ":" + c.Secret
	}
	return c.Secret
}
