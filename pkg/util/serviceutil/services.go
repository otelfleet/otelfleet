package serviceutil

type Module string

func (s Module) Str() string {
	return string(s)
}

func (s Module) Name() string {
	// could use camelcase converter but that seems like overkill.
	switch s {
	case All:
		return "All"
	case Storage:
		return "Storage"
	case Auth:
		return "Authorization"
	case ServerService:
		return "Server"
	case OpAmp:
		return "OpAmp"
	case DeploymentManager:
		return "DeploymentManager"
	case LSP:
		return "LSP"
	case UI:
		return "UI"
	case OTLP:
		return "OTLP"
	case Resource:
		return "DataPlane"
	case Events:
		return "Events"
	case Gateway:
		return "Gateway"
	default:
		return s.Str()
	}
}

func (s Module) TracingService() string {
	return "otelfleet." + s.Name()
}

// The various modules that make up OtelFleet
const (
	All               Module = "all"
	Storage           Module = "storage"
	Auth              Module = "authorization"
	ServerService     Module = "server"
	OpAmp             Module = "opamp"
	DeploymentManager Module = "deployment-manager"
	// DeploymentModule = "deployment"
	LSP Module = "lsp"
	// UI serves the web UI.
	UI Module = "ui"
	// Embedded OTLP service, for connected collector reported self-telemetry
	OTLP Module = "otlp"
	// Resource server is the API over the first class resources in the data layer.
	// Probably worth renaming.
	Resource Module = "resource"
	// Events server is the API responsible for listing/watching events.
	Events Module = "events"
	// Gatewat acts as the control plane service. This is the public
	// entry point for all other services, whether other services
	// run in-process or not
	Gateway Module = "gateway"
)
