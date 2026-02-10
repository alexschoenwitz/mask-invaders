package main

import (
	"testing"
)

func TestUpdateArmyCoordinates(t *testing.T) {
	tests := []struct {
		description  string
		army         *Army
		turn         int
		turnProgress float64
		expectedY    float64
		expectedX    float64
	}{
		{
			description: "Turn of creation, expect starting position",
			army: &Army{
				startX:          100,
				startY:          100,
				targetX:         200,
				targetY:         200,
				turnOfCreation:  66,
				distanceInTurns: 10,
				turnOfArrival:   76,
			},
			turn:         66,
			turnProgress: 0,

			expectedX: 100,
			expectedY: 100,
		},
		{
			description: "Start of first turn, expect position to be 10% of the way to the target",
			army: &Army{
				startX:          100,
				startY:          100,
				targetX:         200,
				targetY:         200,
				turnOfCreation:  66,
				distanceInTurns: 10,
				turnOfArrival:   76,
			},
			turn:         67,
			turnProgress: 0,

			expectedX: 110,
			expectedY: 110,
		},
		{
			description: "Start of third turn, expect position to be 30% of the way to the target",
			army: &Army{
				startX:          100,
				startY:          100,
				targetX:         200,
				targetY:         200,
				turnOfCreation:  66,
				distanceInTurns: 10,
				turnOfArrival:   76,
			},
			turn:         69,
			turnProgress: 0,

			expectedX: 130,
			expectedY: 130,
		},
		{
			description: "Turn of arrival, expect position to be the target position",
			army: &Army{
				startX:          100,
				startY:          100,
				targetX:         200,
				targetY:         200,
				turnOfCreation:  66,
				distanceInTurns: 10,
				turnOfArrival:   76,
			},
			turn:         76,
			turnProgress: 0,

			expectedX: 200,
			expectedY: 200,
		},
		{
			description: "After turn of arrival, expect position to remain at the target position",
			army: &Army{
				startX:          100,
				startY:          100,
				targetX:         200,
				targetY:         200,
				turnOfCreation:  66,
				distanceInTurns: 10,
				turnOfArrival:   76,
			},
			turn:         76,
			turnProgress: 0.1,

			expectedX: 200,
			expectedY: 200,
		},
		{
			description: "Well after turn of arrival, expect position to remain at the target position",
			army: &Army{
				startX:          100,
				startY:          100,
				targetX:         200,
				targetY:         200,
				turnOfCreation:  66,
				distanceInTurns: 10,
				turnOfArrival:   76,
			},
			turn:         78,
			turnProgress: 0,

			expectedX: 200,
			expectedY: 200,
		},
		{
			description: "Shortly after turn of creation, expect position to be slightly moved from the starting position",
			army: &Army{
				startX:          100,
				startY:          100,
				targetX:         200,
				targetY:         200,
				turnOfCreation:  66,
				distanceInTurns: 10,
				turnOfArrival:   76,
			},
			turn:         66,
			turnProgress: 0.1,

			expectedX: 101,
			expectedY: 101,
		},
		{
			description: "Midway through first turn, expect position to be 15% of the way to the target",
			army: &Army{
				startX:          100,
				startY:          100,
				targetX:         200,
				targetY:         200,
				turnOfCreation:  66,
				distanceInTurns: 10,
				turnOfArrival:   76,
			},
			turn:         67,
			turnProgress: 0.5,

			expectedX: 115,
			expectedY: 115,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			tt.army.updateArmyCoordinates(tt.turn, tt.turnProgress)

			if tt.expectedX != tt.army.x {
				t.Errorf("x -> Expected: %f, got %f", tt.expectedX, tt.army.x)
			}
			if tt.expectedY != tt.army.y {
				t.Errorf("y -> Expected: %f, got %f", tt.expectedY, tt.army.y)
			}
		})
	}
}
