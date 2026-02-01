# Advanced Bot for Mask Invaders

A sophisticated AI bot that uses advanced game theory and strategic planning to compete in the Mask Invaders game.

## Features

### Core Capabilities
- **Exact Combat Simulation**: Matches server's battle calculation logic exactly
- **Game State Analysis**: Comprehensive understanding of board position, threats, and opportunities
- **Minimax Search**: 2-ply lookahead with alpha-beta pruning for strategic planning
- **Evaluation Function**: Multi-factor position evaluation considering:
  - City control (most important factor)
  - Material advantage (troop count)
  - Power advantage (quality-adjusted strength)
  - Positional value (strategic positioning)
  - Threat assessment (incoming attacks)

### Strategic Components
1. **Threat Analysis**: Evaluates incoming attacks and determines best defensive response
2. **Target Prioritization**: Scores enemy cities based on winnability, distance, and survival rate
3. **Optimal Troop Composition**: Counters enemy troops using rock-paper-scissors mechanics:
   - Troop A beats C (2.1x effectiveness)
   - Troop B beats A (1.3x effectiveness)
   - Troop C beats B (1.1x effectiveness)
4. **Expansion Strategy**: Prioritizes capturing weak targets in early game
5. **Multi-city Coordination**: Plans coordinated attacks when beneficial

### Decision Making
- **Early Game (Turns 0-4)**: Uses fast heuristic decision making, focuses on expansion
- **Mid/Late Game (Turn 5+)**: Engages minimax search for strategic depth
- **Adaptive**: Switches between aggressive expansion and defensive consolidation based on game state

## Usage

```bash
# Build the bot
go build -o .bin/advanced-bot ./client/advanced-bot

# Run the bot
./.bin/advanced-bot -server http://localhost:8080 -game <GAME_ID> -name "AdvancedBot"
```

### Command-line Arguments
- `-server`: Server URL (default: `http://localhost:8080`)
- `-game`: Game ID to join (required)
- `-name`: Bot name (default: `AdvancedBot`)

## Architecture

```
client/advanced-bot/
├── main.go              # Entry point, game loop, client integration
└── strategy/
    ├── combat.go        # Combat simulation and troop utilities
    ├── state.go         # Game state wrapper with helper methods
    ├── evaluator.go     # State evaluation function
    ├── minimax.go       # Minimax search with alpha-beta pruning
    ├── strategy.go      # Main decision-making logic
    └── tactics.go       # Tactical components (threats, targets, etc.)
```

## How It Works

### Turn Cycle
1. **Get Game State**: Fetch current state from server
2. **Threat Assessment**: Analyze incoming attacks on our cities
3. **Strategic Planning**:
   - If mid/late game: Use minimax to search move tree
   - If early game or minimax unavailable: Use heuristic evaluation
4. **Action Selection**: For each city, decide to:
   - **Attack**: If we can win against a target
   - **Defend**: Build counter-troops if under threat
   - **Build**: Strengthen position for future moves
5. **Submit Actions**: Send all actions to server

### Combat Mechanics
The bot uses the exact battle calculation from the server:
1. Calculate combat effectiveness using troop matchup matrix
2. Compute power = effectiveness × quantity (Linear Law)
3. Winner determined by higher power
4. Survivors calculated proportionally

### Minimax Search
- **Depth**: 2 moves ahead (configurable)
- **Time Budget**: 200ms per decision (leaves 100ms buffer before 300ms turn deadline)
- **Pruning**: Alpha-beta pruning eliminates unpromising branches
- **Move Generation**: Creates 3 strategic move combinations per search:
  1. All cities build troops
  2. Strongest city attacks, others build
  3. Distributed attacks across multiple cities

### Evaluation Weights
- City Control: 100 points per city advantage
- Material Advantage: 10 points per troop advantage
- Power Advantage: 15 points per power unit advantage
- Positional Value: 5 points for strategic positioning
- Threat Penalty: -20 points per threat

## Strategy Summary

**Opening**: Aggressive expansion, capture weak enemy cities quickly

**Mid-game**: Strategic positioning, build up forces, coordinate attacks

**Late-game**: Calculated aggression, use minimax to find winning sequences

**Defense**: Counter-troop composition, only build what's needed to defend

**Attack**: Only attack when simulation predicts victory, prefer high survival rate

## Performance

- **Decision Time**: Typically <100ms for heuristic, <200ms for minimax
- **Memory**: Minimal (no transposition tables or opening books yet)
- **Scalability**: Handles 2-4 player games efficiently

## Future Enhancements

Potential improvements for even stronger play:
- Transposition table for minimax (avoid re-evaluating same positions)
- Iterative deepening (use deeper search when time permits)
- Opening book (pre-computed optimal early moves)
- Monte Carlo Tree Search as alternative to minimax
- Neural network evaluation function
- Multi-agent opponent modeling

## Testing

The bot has been designed to:
- Make valid moves (respects game rules)
- Decide within time constraints (<300ms per turn)
- Handle edge cases (losing cities, no valid moves, etc.)
- Adapt to different game situations

To test against other bots, run multiple instances with different game IDs or names.
