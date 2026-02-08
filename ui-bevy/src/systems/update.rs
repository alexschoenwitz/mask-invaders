use bevy::prelude::*;
use std::time::{Duration, Instant};

use crate::http_client::HttpClient;
use crate::resources::{
    AnimationState, BufferedState, CityPositions, DropdownState, GameConfig, GameMode,
    MovementStartCache, PlayerRegistry, StateBuffer, StateHistory,
};

const POLL_INTERVAL: Duration = Duration::from_millis(100);

pub fn update_animation(
    time: Res<Time>,
    config: Res<GameConfig>,
    state_history: Res<StateHistory>,
    state_buffer: Res<StateBuffer>,
    mut animation: ResMut<AnimationState>,
) {
    animation.tick_counter = animation.tick_counter.wrapping_add(1);

    let now = Instant::now();

    if config.mode == GameMode::Replay {
        update_replay_mode(&time, &config, &state_history, &mut animation, now);
    } else {
        update_live_mode(&config, &state_buffer, &mut animation, now);
    }
}

fn update_replay_mode(
    _time: &Time,
    config: &GameConfig,
    state_history: &StateHistory,
    animation: &mut AnimationState,
    now: Instant,
) {
    if animation.last_update.is_none() {
        animation.last_update = Some(now);
    }

    let last_update = animation.last_update.unwrap();
    let elapsed = now.duration_since(last_update);
    animation.turn_progress = elapsed.as_secs_f64() / config.turn_duration.as_secs_f64();

    if !state_history.states.is_empty() && animation.current_state_idx < state_history.states.len()
    {
        let base_turn = state_history.states[animation.current_state_idx].turn as f64;
        animation.current_turn = base_turn + animation.turn_progress;
    }

    if elapsed >= config.turn_duration {
        animation.last_update = Some(now);
        animation.turn_progress = 0.0;
        animation.current_state_idx += 1;
        if animation.current_state_idx >= state_history.states.len() {
            animation.current_state_idx = state_history.states.len().saturating_sub(1);
        }
    }
}

fn update_live_mode(
    config: &GameConfig,
    state_buffer: &StateBuffer,
    animation: &mut AnimationState,
    now: Instant,
) {
    if state_buffer.buffer.len() < StateBuffer::MIN_BUFFER_STATES {
        return;
    }

    if animation.display_state_idx < 0 {
        animation.display_state_idx = 0;
        animation.current_state_idx = 0;
        animation.animation_start = Some(now);
        animation.current_turn = state_buffer.buffer[0].state.turn as f64;
        info!(
            "Started playback from turn {} with {} states buffered",
            state_buffer.buffer[0].state.turn,
            state_buffer.buffer.len()
        );
        return;
    }

    let animation_start = animation.animation_start.unwrap_or(now);
    let elapsed = now.duration_since(animation_start);

    if elapsed >= config.turn_duration {
        let next_state_idx = animation.display_state_idx + 1;
        if (next_state_idx as usize) < state_buffer.buffer.len() {
            animation.display_state_idx = next_state_idx;
            animation.current_state_idx = next_state_idx as usize;
            animation.animation_start = Some(animation_start + config.turn_duration);

            let states_ahead = state_buffer.buffer.len() - animation.display_state_idx as usize - 1;
            info!(
                "Advanced to turn {}, {} states buffered ahead",
                state_buffer.buffer[next_state_idx as usize].state.turn, states_ahead
            );
        }
    }

    if animation.display_state_idx >= 0
        && (animation.display_state_idx as usize) < state_buffer.buffer.len()
    {
        let current_state = &state_buffer.buffer[animation.display_state_idx as usize].state;
        let current_turn = current_state.turn as f64;

        let animation_start = animation.animation_start.unwrap_or(now);
        let elapsed = now.duration_since(animation_start);
        let t = (elapsed.as_secs_f64() / config.turn_duration.as_secs_f64()).min(1.0);

        if (animation.display_state_idx as usize + 1) < state_buffer.buffer.len() {
            let next_turn =
                state_buffer.buffer[animation.display_state_idx as usize + 1].state.turn as f64;
            animation.current_turn = current_turn + (next_turn - current_turn) * t;
        } else {
            animation.current_turn = current_turn;
        }

        animation.turn_progress = t;
    }
}

pub fn poll_server(
    config: Res<GameConfig>,
    dropdown: Res<DropdownState>,
    mut state_buffer: ResMut<StateBuffer>,
    mut state_history: ResMut<StateHistory>,
    mut animation: ResMut<AnimationState>,
    mut city_positions: ResMut<CityPositions>,
    mut player_registry: ResMut<PlayerRegistry>,
    mut movement_cache: ResMut<MovementStartCache>,
) {
    if config.mode != GameMode::Live {
        return;
    }

    let now = Instant::now();
    if let Some(last_poll) = state_buffer.last_poll {
        if now.duration_since(last_poll) < POLL_INTERVAL {
            return;
        }
    }
    state_buffer.last_poll = Some(now);

    let Some(ref game_id) = dropdown.selected_game else {
        return;
    };

    let client = HttpClient::new(&config.api_url);
    let state = match client.fetch_state(game_id) {
        Ok(Some(state)) => state,
        Ok(None) => return,
        Err(e) => {
            warn!("Failed to poll server: {}", e);
            return;
        }
    };

    if !state_buffer.buffer.is_empty()
        && state.turn < state_buffer.buffer.last().unwrap().state.turn
    {
        info!(
            "New game detected (turn {} < previous {}), resetting state",
            state.turn,
            state_buffer.buffer.last().unwrap().state.turn
        );
        state_history.states.clear();
        state_buffer.buffer.clear();
        city_positions.positions.clear();
        player_registry.colors.clear();
        player_registry.names.clear();
        player_registry.player_list.clear();
        animation.current_state_idx = 0;
        animation.display_state_idx = -1;
        animation.current_turn = 0.0;
        animation.animation_start = None;
        movement_cache.cache.clear();
    }

    let is_new_state =
        state_buffer.buffer.is_empty() || state_buffer.buffer.last().unwrap().state.turn != state.turn;

    if is_new_state {
        let buffered = BufferedState {
            state: state.clone(),
            received_at: now,
        };
        state_buffer.buffer.push(buffered);

        if state_buffer.buffer.len() > StateBuffer::STATE_BUFFER_SIZE {
            state_buffer.buffer.remove(0);
            if animation.display_state_idx > 0 {
                animation.display_state_idx -= 1;
            }
        }

        state_history.states.push(state.clone());

        info!(
            "Buffered state for turn {}, buffer size: {}",
            state.turn,
            state_buffer.buffer.len()
        );
    }
}
