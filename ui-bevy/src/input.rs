use bevy::prelude::*;

use crate::http_client::HttpClient;
use crate::resources::{
    AnimationState, CityPositions, DropdownState, GameConfig, GameMode, MovementStartCache,
    PlayerRegistry, StateBuffer, StateHistory,
};

pub fn handle_dropdown_input(
    mouse_button: Res<ButtonInput<MouseButton>>,
    windows: Query<&Window>,
    mut dropdown: ResMut<DropdownState>,
    config: Res<GameConfig>,
    mut state_history: ResMut<StateHistory>,
    mut state_buffer: ResMut<StateBuffer>,
    mut animation_state: ResMut<AnimationState>,
    mut player_registry: ResMut<PlayerRegistry>,
    mut city_positions: ResMut<CityPositions>,
    mut movement_cache: ResMut<MovementStartCache>,
) {
    if config.mode != GameMode::Live {
        return;
    }

    if !mouse_button.just_pressed(MouseButton::Left) {
        return;
    }

    let window = windows.single();
    let Some(cursor_position) = window.cursor_position() else {
        return;
    };

    let dropdown_x = 10.0;
    let dropdown_width = 200.0;
    let dropdown_height = 25.0;
    let dropdown_y = dropdown.y_position;

    let mx = cursor_position.x;
    let my = cursor_position.y;

    if mx >= dropdown_x
        && mx <= dropdown_x + dropdown_width
        && my >= dropdown_y
        && my <= dropdown_y + dropdown_height
    {
        if !dropdown.expanded {
            dropdown.expanded = true;
            if let Ok(games) = HttpClient::new(&config.api_url).fetch_game_list() {
                dropdown.available_games = games;
            }
        }
        return;
    }

    if dropdown.expanded {
        let option_height = 20.0;
        let available_games = dropdown.available_games.clone();
        let mut selected_game_id: Option<String> = None;

        for (i, game_id) in available_games.iter().enumerate() {
            let option_y = dropdown_y + dropdown_height + (i as f32) * option_height;

            if mx >= dropdown_x
                && mx <= dropdown_x + dropdown_width
                && my >= option_y
                && my <= option_y + option_height
            {
                if dropdown.selected_game.as_ref() != Some(game_id) {
                    selected_game_id = Some(game_id.clone());
                }
                dropdown.expanded = false;
                break;
            }
        }

        if let Some(game_id) = selected_game_id {
            info!("Selected game: {}", game_id);
            dropdown.selected_game = Some(game_id.clone());

            state_history.states.clear();
            state_buffer.buffer.clear();
            animation_state.current_state_idx = 0;
            animation_state.display_state_idx = -1;
            animation_state.current_turn = 0.0;
            animation_state.animation_start = None;
            player_registry.colors.clear();
            player_registry.names.clear();
            player_registry.player_list.clear();
            city_positions.positions.clear();
            movement_cache.cache.clear();

            let client = HttpClient::new(&config.api_url);
            if let Ok(states) = client.fetch_state_history(&game_id) {
                state_history.states = states;
                info!(
                    "Fetched {} historical states for game {}",
                    state_history.states.len(),
                    game_id
                );
            }
        } else if selected_game_id.is_none() {
            dropdown.expanded = false;
        }
    }
}
