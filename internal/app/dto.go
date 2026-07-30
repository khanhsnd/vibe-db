package app

import "github.com/khanhsnd/vibe-db/internal/domain"

type Profile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Engine       string `json:"engine"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Database     string `json:"database"`
	TLSMode      string `json:"tlsMode"`
	SavePassword bool   `json:"savePassword"`
	HasPassword  bool   `json:"hasPassword"`
}

type ProfileInput struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Password     string `json:"password"`
	Database     string `json:"database"`
	TLSMode      string `json:"tlsMode"`
	SavePassword bool   `json:"savePassword"`
}

type TestResult struct {
	ServerVersion string `json:"serverVersion"`
	LatencyMs     int64  `json:"latencyMs"`
}

type Session struct {
	ID            string `json:"id"`
	ProfileID     string `json:"profileId"`
	Target        string `json:"target"`
	ServerVersion string `json:"serverVersion"`
}

func toProfileDTO(profile domain.ConnectionProfile, hasPassword bool) Profile {
	return Profile{
		ID:           profile.ID,
		Name:         profile.Name,
		Engine:       string(profile.Engine),
		Host:         profile.Host,
		Port:         profile.Port,
		User:         profile.User,
		Database:     profile.Database,
		TLSMode:      string(profile.TLSMode),
		SavePassword: profile.SavePassword,
		HasPassword:  hasPassword,
	}
}

func (in ProfileInput) toDomain() domain.ConnectionProfile {
	port := in.Port
	if port == 0 {
		port = domain.DefaultMySQLPort
	}

	tlsMode := domain.TLSMode(in.TLSMode)
	if tlsMode == "" {
		tlsMode = domain.TLSPreferred
	}

	return domain.ConnectionProfile{
		ID:           in.ID,
		Name:         in.Name,
		Engine:       domain.EngineMySQL,
		Host:         in.Host,
		Port:         port,
		User:         in.User,
		Database:     in.Database,
		TLSMode:      tlsMode,
		SavePassword: in.SavePassword,
	}
}
