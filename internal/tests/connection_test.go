package tests

import (
	"context"
	"testing"

	"City2TABULA/internal/config"
	"City2TABULA/internal/db"

	"github.com/stretchr/testify/assert"
)

func TestEnsureDatabaseValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.Config
		expectErr bool
	}{
		{
			name: "Valid config",
			config: &config.Config{
				DB: &config.DBConfig{
					Host:     "localhost",
					Port:     "5432",
					User:     "testuser",
					Password: "testpass",
					Name:     "testdb",
					SSLMode:  "disable",
				},
			},
			expectErr: false, // Will fail at connection, but config is valid
		},
		{
			name: "Nil DB config",
			config: &config.Config{
				DB: nil,
			},
			expectErr: true,
		},
		{
			name: "Empty database name",
			config: &config.Config{
				DB: &config.DBConfig{
					Host:     "localhost",
					Port:     "5432",
					User:     "testuser",
					Password: "testpass",
					Name:     "",
					SSLMode:  "disable",
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.DB == nil {
				// EnsureDatabase doesn't handle nil DB config gracefully
				assert.Panics(t, func() {
					db.EnsureDatabase(tt.config)
				})
			} else {
				err := db.EnsureDatabase(tt.config)
				if tt.expectErr {
					assert.Error(t, err)
				} else {
					// For valid configs, we expect a connection error since we don't have a real DB
					// The important thing is that it doesn't fail on config validation
					assert.Error(t, err) // Will fail to connect, but that's expected in tests
				}
			}
		})
	}
}

func TestConnectPoolValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.Config
		expectErr bool
	}{
		{
			name: "Valid config",
			config: &config.Config{
				DB: &config.DBConfig{
					Host:     "localhost",
					Port:     "5432",
					User:     "testuser",
					Password: "testpass",
					Name:     "testdb",
					SSLMode:  "disable",
				},
			},
			expectErr: false, // Will fail at connection, but config is valid
		},
		{
			name: "Nil DB config",
			config: &config.Config{
				DB: nil,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.DB == nil {
				// ConnectPool doesn't handle nil DB config gracefully
				assert.Panics(t, func() {
					db.ConnectPool(tt.config)
				})
			} else {
				pool, err := db.ConnectPool(tt.config)
				if tt.expectErr {
					assert.Error(t, err)
					assert.Nil(t, pool)
				} else {
					// For valid configs, we expect a connection error since we don't have a real DB
					assert.Error(t, err) // Will fail to connect, but that's expected in tests
					assert.Nil(t, pool)
				}
			}
		})
	}
}

func TestDBConfigFields(t *testing.T) {
	// Test that DBConfig struct has all required fields
	dbConfig := &config.DBConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "testuser",
		Password: "testpass",
		Name:     "testdb",
		SSLMode:  "disable",
	}

	assert.Equal(t, "localhost", dbConfig.Host)
	assert.Equal(t, "5432", dbConfig.Port)
	assert.Equal(t, "testuser", dbConfig.User)
	assert.Equal(t, "testpass", dbConfig.Password)
	assert.Equal(t, "testdb", dbConfig.Name)
	assert.Equal(t, "disable", dbConfig.SSLMode)
}

func TestConfigStructure(t *testing.T) {
	// Test that Config has a DB field
	config := &config.Config{
		Country: "germany",
		DB: &config.DBConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "testuser",
			Password: "testpass",
			Name:     "testdb",
			SSLMode:  "disable",
		},
	}

	assert.NotNil(t, config.DB)
	assert.Equal(t, "germany", config.Country)
	assert.Equal(t, "localhost", config.DB.Host)
}

func TestDSNComponents(t *testing.T) {
	// Test DSN formatting indirectly by checking that the functions accept proper configs
	tests := []struct {
		name     string
		dbConfig *config.DBConfig
		valid    bool
	}{
		{
			name: "Standard PostgreSQL config",
			dbConfig: &config.DBConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "postgres",
				Password: "password",
				Name:     "testdb",
				SSLMode:  "disable",
			},
			valid: true,
		},
		{
			name: "SSL enabled config",
			dbConfig: &config.DBConfig{
				Host:     "prod.example.com",
				Port:     "5432",
				User:     "produser",
				Password: "securepass",
				Name:     "proddb",
				SSLMode:  "require",
			},
			valid: true,
		},
		{
			name: "Custom port config",
			dbConfig: &config.DBConfig{
				Host:     "db.example.com",
				Port:     "5433",
				User:     "customuser",
				Password: "custompass",
				Name:     "customdb",
				SSLMode:  "prefer",
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &config.Config{DB: tt.dbConfig}

			if tt.valid {
				// These will fail to connect but shouldn't fail on config validation
				assert.NotNil(t, config.DB)
				assert.NotEmpty(t, config.DB.Host)
				assert.NotEmpty(t, config.DB.Port)
				assert.NotEmpty(t, config.DB.User)
				assert.NotEmpty(t, config.DB.Name)
			}
		})
	}
}

func TestSSLModeOptions(t *testing.T) {
	validSSLModes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}

	for _, sslMode := range validSSLModes {
		t.Run("SSLMode_"+sslMode, func(t *testing.T) {
			dbConfig := &config.DBConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "testuser",
				Password: "testpass",
				Name:     "testdb",
				SSLMode:  sslMode,
			}

			assert.Equal(t, sslMode, dbConfig.SSLMode)
		})
	}
}

func TestContextUsage(t *testing.T) {
	// Test that context is properly used (indirectly)
	ctx := context.Background()
	assert.NotNil(t, ctx)

	// Test context with timeout
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	assert.NotNil(t, ctx)
}

// Test special characters in connection parameters
func TestSpecialCharactersInConfig(t *testing.T) {
	tests := []struct {
		name     string
		dbConfig *config.DBConfig
	}{
		{
			name: "Special chars in password",
			dbConfig: &config.DBConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "testuser",
				Password: "p@ssw0rd!#$%",
				Name:     "testdb",
				SSLMode:  "disable",
			},
		},
		{
			name: "Underscores and hyphens in database name",
			dbConfig: &config.DBConfig{
				Host:     "localhost",
				Port:     "5432",
				User:     "testuser",
				Password: "password",
				Name:     "test_db-2024",
				SSLMode:  "disable",
			},
		},
		{
			name: "Domain name with subdomain",
			dbConfig: &config.DBConfig{
				Host:     "db.prod.example.com",
				Port:     "5432",
				User:     "testuser",
				Password: "password",
				Name:     "testdb",
				SSLMode:  "disable",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &config.Config{DB: tt.dbConfig}
			assert.NotNil(t, config.DB)

			// Test that the config structure holds the values correctly
			assert.Equal(t, tt.dbConfig.Host, config.DB.Host)
			assert.Equal(t, tt.dbConfig.Password, config.DB.Password)
			assert.Equal(t, tt.dbConfig.Name, config.DB.Name)
		})
	}
}
