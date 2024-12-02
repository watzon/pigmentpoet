package config

// Config holds all configuration for the bot
type Config struct {
	// Bluesky credentials
	Handle   string
	Password string

	// Output directory for generated images
	OutputDir string

	// Image configuration
	MaxWidth  int
	MaxHeight int
}

// DefaultConfig returns a new Config with default values
func DefaultConfig() *Config {
	return &Config{
		MaxWidth:  1600,
		MaxHeight: 1600,
	}
}

// WithHandle sets the Bluesky handle
func (c *Config) WithHandle(handle string) *Config {
	c.Handle = handle
	return c
}

// WithPassword sets the Bluesky password
func (c *Config) WithPassword(password string) *Config {
	c.Password = password
	return c
}

// WithOutputDir sets the output directory
func (c *Config) WithOutputDir(dir string) *Config {
	c.OutputDir = dir
	return c
}
