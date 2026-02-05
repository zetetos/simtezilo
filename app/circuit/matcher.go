package circuit

import (
	"math"

	"github.com/zetetos/gt-telemetry/v2/pkg/circuits"
)

// Candidate represents a potential circuit match with confidence tracking.
type Candidate struct {
	info          circuits.CircuitInfo // Circuit information
	matchedCoords map[string]bool      // Set of matched coordinates (as string keys)
	confidence    float64              // Confidence level (0.0 to 1.0)
}

// Candidates represents a collection of matching circuit candidates.
type Candidates map[string]*Candidate

// updateCandidateConfidence updates the match confidence for a circuit name based on a given coordinate key.
func (c *Circuit) updateCandidateConfidence(circuitID string, coordinateKey string) {
	candidate := c.getCandidate(circuitID)
	if candidate == nil {
		return
	}

	if !candidate.matchedCoords[coordinateKey] {
		candidate.matchedCoords[coordinateKey] = true
	}

	matchCount := len(candidate.matchedCoords)
	totalCoords := candidate.info.UniqueCoordinateCount

	if totalCoords <= 0 {
		return
	}

	candidate.confidence = min(float64(matchCount)/float64(totalCoords), 1.0)

	c.log.Debug().
		Str("circuit", candidate.info.Variation).
		Int("confidence", int(math.Round(candidate.confidence*100))).
		Int("matches", len(candidate.matchedCoords)).
		Int("total_coords", candidate.info.UniqueCoordinateCount).
		Msg("Updated circuit confidence")
}

// getCandidate gets an existing candidate or creates a new one.
func (c *Circuit) getCandidate(circuitID string) *Candidate {
	if candidate, exists := c.candidates[circuitID]; exists {
		return candidate
	}

	circuitInfo, found := c.database.GetCircuitByID(circuitID)
	if !found {
		c.log.Error().
			Str("circuit_id", circuitID).
			Msg("Circuit not found in database")

		return nil
	}

	candidate := &Candidate{
		info:          circuitInfo,
		matchedCoords: make(map[string]bool),
		confidence:    0.0,
	}

	c.candidates[circuitID] = candidate

	return candidate
}

// bestCandidate returns the circuit candidate with the highest confidence above threshold.
func (c *Circuit) bestCandidate() *Candidate {
	var bestCandidate *Candidate

	highestConfidence := float64(0.0)

	for _, candidate := range c.candidates {
		if candidate.confidence < minConfidenceThreshold {
			continue
		}

		if candidate.confidence > highestConfidence {
			highestConfidence = candidate.confidence
			bestCandidate = candidate
		}
	}

	return bestCandidate
}
