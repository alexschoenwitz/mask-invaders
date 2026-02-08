use bevy::prelude::*;
use std::collections::HashMap;

#[derive(Component)]
pub struct City {
    pub id: String,
    pub player: String,
    pub troops: HashMap<String, i64>,
    pub base_position: Vec2,
}

#[derive(Component)]
pub struct CityTroopText {
    pub city_id: String,
}

#[derive(Component)]
pub struct Movement {
    pub from_city: String,
    pub to_city: String,
    pub troops: HashMap<String, i64>,
    pub player: String,
    pub arriving_turn: i64,
    pub start_turn: i64,
    pub progress: f64,
}

impl Movement {
    pub fn id(&self) -> String {
        format!(
            "{}->{}@{}:{}",
            self.from_city, self.to_city, self.arriving_turn, self.player
        )
    }
}

#[derive(Component)]
pub struct TroopSprite {
    pub troop_type: String,
    pub player: String,
}

#[derive(Component)]
pub struct Sun;

#[derive(Component)]
pub struct Moon;

#[derive(Component)]
pub struct Sky;

#[derive(Component)]
pub struct Ground;

#[derive(Component)]
pub struct CastleBody {
    pub city_id: String,
}

#[derive(Component)]
pub struct CastleTower {
    pub city_id: String,
    pub tower_index: usize,
}

#[derive(Component)]
pub struct CastleGate {
    pub city_id: String,
}

#[derive(Component)]
pub struct StatsGraph;

#[derive(Component)]
pub struct GraphLine {
    pub player: String,
}

#[derive(Component)]
pub struct GraphLegend;

#[derive(Component)]
pub struct TurnCounter;

#[derive(Component)]
pub struct ModeIndicator;

#[derive(Component)]
pub struct DropdownHeader;

#[derive(Component)]
pub struct DropdownOption {
    pub game_id: String,
}

#[derive(Component)]
pub struct MovementTroop {
    pub movement_id: String,
    pub troop_type: String,
    pub offset_index: i32,
}
