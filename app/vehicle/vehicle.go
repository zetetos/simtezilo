package vehicle

import (
	"github.com/zetetos/simtezilo/app/haptics"
)

type TypeName string

const (
	TypeStreet TypeName = "street"
	TypeTuned  TypeName = "tuned"
	TypeRace   TypeName = "race"
)

type EngineCharacteristics struct {
	Layout          string
	DBEntry         string
	Geometry        string
	Chambers        int
	RevLimit        uint16
	FiringFrequency float64
	PulseOverlap    float64 // Calculated overlap factor based on cylinder/crank alignment
	Haptics         *haptics.EngineProfile
}

type Dimensions struct {
	WheelbaseMetres    float32 // Distance between front and rear axles
	TrackWidthMetres   float32 // Distance between left and right wheels on the same axle
	LongitudinalRadius float32 // Fore/Aft rotational radius in metres (wheelbase / 2)
	TransverseRadius   float32 // Lateral rotational radius in metres (track width / 2)
}

// Characteristics holds static vehicle data loaded from the GT vehicle database.
type Characteristics struct {
	ID          uint32                // Unique vehicle ID from telemetry
	VehicleType TypeName              // Vehicle type
	Engine      EngineCharacteristics // Engine characteristics
	RevLimit    uint16                // Engine rev limit in RPM
	Dimensions  Dimensions            // Vehicle dimensions
}

// DetermineVehicleType determines the vehicle type from a given vehicle type string.
func DetermineVehicleType(vehicleType string) TypeName {
	switch vehicleType {
	case string(TypeStreet):
		return TypeStreet
	case string(TypeTuned):
		return TypeTuned
	case string(TypeRace):
		return TypeRace
	default:
		return TypeStreet
	}
}
