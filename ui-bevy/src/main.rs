use bevy::prelude::*;
use bevy::window::WindowResolution;
use bevy_prototype_lyon::prelude::*;
use clap::Parser;
use std::time::Duration;

mod components;
mod data;
mod http_client;
mod input;
mod resources;
mod systems;

use http_client::{load_replay_file, HttpClient};
use resources::*;
use systems::*;

#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
struct Args {
    #[arg(help = "Path to replay JSON file (optional)")]
    replay_file: Option<String>,

    #[arg(short, long, default_value = "http://localhost:8080", help = "Server URL")]
    server: String,

    #[arg(long, default_value = "300", help = "Turn duration in ms")]
    turn_duration_ms: u64,
}

fn main() {
    let args = Args::parse();

    let mode = if args.replay_file.is_some() {
        GameMode::Replay
    } else {
        GameMode::Live
    };

    let config = GameConfig {
        mode,
        api_url: args.server.clone(),
        turn_duration: Duration::from_millis(args.turn_duration_ms),
        replay_file: args.replay_file.clone(),
    };

    let mut app = App::new();

    app.add_plugins(DefaultPlugins.set(WindowPlugin {
        primary_window: Some(Window {
            title: "Mask Invaders Visualization".to_string(),
            resolution: WindowResolution::new(800.0, 800.0),
            resizable: true,
            ..default()
        }),
        ..default()
    }))
    .add_plugins(ShapePlugin)
    .insert_resource(config)
    .insert_resource(AnimationState::default())
    .insert_resource(StateBuffer::default())
    .insert_resource(StateHistory::default())
    .insert_resource(PlayerRegistry::default())
    .insert_resource(DropdownState::new())
    .insert_resource(CityPositions::default())
    .insert_resource(MovementStartCache::default())
    .insert_resource(ScreenDimensions::default())
    .add_systems(Startup, setup_camera)
    .add_systems(Startup, setup_background)
    .add_systems(Startup, setup_graph)
    .add_systems(Startup, setup_ui)
    .add_systems(Startup, setup_dropdown)
    .add_systems(Startup, load_initial_state)
    .add_systems(
        Update,
        (
            update_animation,
            poll_server,
            initialize_cities,
            update_cities,
            update_movements,
            update_day_night_cycle,
            update_celestial_bodies,
            update_graph,
            update_turn_counter,
            update_dropdown,
            render_dropdown_options,
            input::handle_dropdown_input,
        ),
    );

    app.run();
}

fn load_initial_state(
    config: Res<GameConfig>,
    mut state_history: ResMut<StateHistory>,
    mut dropdown: ResMut<DropdownState>,
) {
    match config.mode {
        GameMode::Replay => {
            if let Some(ref path) = config.replay_file {
                match load_replay_file(path) {
                    Ok(states) => {
                        info!("Loaded {} states from replay file", states.len());
                        state_history.states = states;
                    }
                    Err(e) => {
                        error!("Failed to load replay file: {}", e);
                    }
                }
            }
        }
        GameMode::Live => {
            let client = HttpClient::new(&config.api_url);

            match client.fetch_game_list() {
                Ok(games) => {
                    info!("Fetched {} available games", games.len());
                    dropdown.available_games = games.clone();

                    if let Some(first_game) = games.first() {
                        dropdown.selected_game = Some(first_game.clone());
                        info!("Selected default game: {}", first_game);

                        if let Ok(states) = client.fetch_state_history(first_game) {
                            info!("Fetched {} historical states", states.len());
                            state_history.states = states;
                        }
                    }
                }
                Err(e) => {
                    warn!("Failed to fetch game list: {}", e);
                }
            }
        }
    }
}
