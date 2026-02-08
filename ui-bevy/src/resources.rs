use bevy::prelude::*;
use std::collections::HashMap;
use std::time::{Duration, Instant};

use crate::data::State;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum GameMode {
    #[default]
    Live,
    Replay,
}

#[derive(Resource)]
pub struct GameConfig {
    pub mode: GameMode,
    pub api_url: String,
    pub turn_duration: Duration,
    pub replay_file: Option<String>,
}

impl Default for GameConfig {
    fn default() -> Self {
        Self {
            mode: GameMode::Live,
            api_url: "http://localhost:8080".to_string(),
            turn_duration: Duration::from_millis(300),
            replay_file: None,
        }
    }
}

#[derive(Resource, Default)]
pub struct AnimationState {
    pub current_turn: f64,
    pub turn_progress: f64,
    pub current_state_idx: usize,
    pub display_state_idx: i32,
    pub last_update: Option<Instant>,
    pub animation_start: Option<Instant>,
    pub tick_counter: u32,
}

#[derive(Clone)]
pub struct BufferedState {
    pub state: State,
    pub received_at: Instant,
}

#[derive(Resource, Default)]
pub struct StateBuffer {
    pub buffer: Vec<BufferedState>,
    pub last_poll: Option<Instant>,
}

impl StateBuffer {
    pub const MIN_BUFFER_STATES: usize = 3;
    pub const STATE_BUFFER_SIZE: usize = 20;
}

#[derive(Resource, Default)]
pub struct StateHistory {
    pub states: Vec<State>,
}

#[derive(Resource, Default)]
pub struct PlayerRegistry {
    pub colors: HashMap<String, Color>,
    pub names: HashMap<String, String>,
    pub player_list: Vec<String>,
}

impl PlayerRegistry {
    pub fn color_palette() -> Vec<Color> {
        vec![
            Color::srgba(200.0 / 255.0, 80.0 / 255.0, 80.0 / 255.0, 1.0),   // Soft Red
            Color::srgba(80.0 / 255.0, 180.0 / 255.0, 80.0 / 255.0, 1.0),  // Soft Green
            Color::srgba(80.0 / 255.0, 120.0 / 255.0, 200.0 / 255.0, 1.0), // Soft Blue
            Color::srgba(220.0 / 255.0, 200.0 / 255.0, 80.0 / 255.0, 1.0), // Soft Yellow
            Color::srgba(200.0 / 255.0, 100.0 / 255.0, 200.0 / 255.0, 1.0), // Soft Magenta
            Color::srgba(80.0 / 255.0, 200.0 / 255.0, 200.0 / 255.0, 1.0), // Soft Cyan
            Color::srgba(220.0 / 255.0, 140.0 / 255.0, 80.0 / 255.0, 1.0), // Soft Orange
            Color::srgba(150.0 / 255.0, 100.0 / 255.0, 200.0 / 255.0, 1.0), // Soft Purple
            Color::srgba(220.0 / 255.0, 160.0 / 255.0, 180.0 / 255.0, 1.0), // Soft Pink
            Color::srgba(100.0 / 255.0, 150.0 / 255.0, 100.0 / 255.0, 1.0), // Soft Dark Green
            Color::srgba(150.0 / 255.0, 150.0 / 255.0, 150.0 / 255.0, 1.0), // Soft Gray
            Color::srgba(220.0 / 255.0, 220.0 / 255.0, 220.0 / 255.0, 1.0), // Soft White
            Color::srgba(150.0 / 255.0, 80.0 / 255.0, 80.0 / 255.0, 1.0),  // Soft Maroon
            Color::srgba(80.0 / 255.0, 150.0 / 255.0, 150.0 / 255.0, 1.0), // Soft Teal
            Color::srgba(128.0 / 255.0, 128.0 / 255.0, 0.0 / 255.0, 1.0),  // Olive
            Color::srgba(0.0 / 255.0, 0.0 / 255.0, 128.0 / 255.0, 1.0),    // Navy
        ]
    }

    pub fn get_color(&self, player: &str) -> Color {
        self.colors
            .get(player)
            .copied()
            .unwrap_or(Color::srgba(0.5, 0.5, 0.5, 1.0))
    }

    pub fn get_display_name<'a>(&'a self, player: &'a str) -> &'a str {
        self.names.get(player).map(|s| s.as_str()).unwrap_or(player)
    }
}

#[derive(Resource, Default)]
pub struct DropdownState {
    pub expanded: bool,
    pub selected_game: Option<String>,
    pub available_games: Vec<String>,
    pub y_position: f32,
}

impl DropdownState {
    pub fn new() -> Self {
        Self {
            expanded: false,
            selected_game: None,
            available_games: Vec::new(),
            y_position: 20.0,
        }
    }
}

#[derive(Resource, Default)]
pub struct CityPositions {
    pub positions: HashMap<String, Vec2>,
}

#[derive(Resource, Default)]
pub struct MovementStartCache {
    pub cache: HashMap<String, i64>,
}

#[derive(Resource)]
pub struct ScreenDimensions {
    pub width: f32,
    pub height: f32,
}

impl Default for ScreenDimensions {
    fn default() -> Self {
        Self {
            width: 800.0,
            height: 800.0,
        }
    }
}
