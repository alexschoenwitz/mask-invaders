use bevy::prelude::*;
use bevy_prototype_lyon::prelude::*;
use std::collections::HashMap;
use std::f32::consts::PI;

use crate::components::{CastleBody, CastleGate, CastleTower, City, CityTroopText};
use crate::data::State;
use crate::resources::{
    AnimationState, CityPositions, GameConfig, GameMode, PlayerRegistry, ScreenDimensions,
    StateBuffer, StateHistory,
};

pub fn initialize_cities(
    mut commands: Commands,
    state_history: Res<StateHistory>,
    mut city_positions: ResMut<CityPositions>,
    mut player_registry: ResMut<PlayerRegistry>,
    screen: Res<ScreenDimensions>,
    existing_cities: Query<Entity, With<City>>,
    config: Res<GameConfig>,
) {
    if !city_positions.positions.is_empty() {
        return;
    }

    let states = if config.mode == GameMode::Replay {
        &state_history.states
    } else if !state_history.states.is_empty() {
        &state_history.states
    } else {
        return;
    };

    if states.is_empty() {
        return;
    }

    for entity in existing_cities.iter() {
        commands.entity(entity).despawn_recursive();
    }

    let first_state = &states[0];
    let num_cities = first_state.cities.len();
    if num_cities == 0 {
        return;
    }

    let center_x = 0.0;
    let center_y = 0.0;
    let radius = screen.width.min(screen.height) * 0.3;

    for (i, city_name) in first_state.cities.keys().enumerate() {
        let (x, y) = if num_cities == 1 {
            (center_x, center_y)
        } else {
            let angle = 2.0 * PI * (i as f32) / (num_cities as f32);
            (center_x + radius * angle.cos(), center_y + radius * angle.sin())
        };

        city_positions.positions.insert(city_name.clone(), Vec2::new(x, y));
    }

    assign_player_colors(states, &mut player_registry);

    for (city_name, city_data) in &first_state.cities {
        let pos = city_positions.positions.get(city_name).unwrap();
        spawn_city(&mut commands, city_name, city_data.player.clone(), city_data.troops.clone(), *pos, &player_registry);
    }
}

fn assign_player_colors(states: &[State], player_registry: &mut PlayerRegistry) {
    let mut player_set: std::collections::HashSet<String> = std::collections::HashSet::new();

    for state in states {
        for (player_id, player_name) in &state.player_names {
            player_registry.names.insert(player_id.clone(), player_name.clone());
        }

        for city in state.cities.values() {
            player_set.insert(city.player.clone());
        }
        for movement in &state.movements {
            player_set.insert(movement.player.clone());
        }
    }

    player_registry.player_list = player_set.into_iter().collect();
    player_registry.player_list.sort();

    let palette = PlayerRegistry::color_palette();
    for (i, player) in player_registry.player_list.iter().enumerate() {
        let color_idx = i % palette.len();
        player_registry.colors.insert(player.clone(), palette[color_idx]);
    }
}

fn spawn_city(
    commands: &mut Commands,
    city_name: &str,
    player: String,
    troops: HashMap<String, i64>,
    pos: Vec2,
    player_registry: &PlayerRegistry,
) {
    let castle_size = 20.0;
    let player_color = player_registry.get_color(&player);

    let body_color = Color::srgba(
        player_color.to_srgba().red * 0.9,
        player_color.to_srgba().green * 0.9,
        player_color.to_srgba().blue * 0.9,
        1.0,
    );

    let body_shape = shapes::Rectangle {
        extents: Vec2::new(castle_size, castle_size * 0.75),
        origin: RectangleOrigin::Center,
        ..default()
    };

    commands
        .spawn((
            City {
                id: city_name.to_string(),
                player: player.clone(),
                troops: troops.clone(),
                base_position: pos,
            },
            Transform::from_xyz(pos.x, pos.y, 5.0),
            Visibility::Visible,
        ))
        .with_children(|parent| {
            parent.spawn((
                CastleBody {
                    city_id: city_name.to_string(),
                },
                ShapeBundle {
                    path: GeometryBuilder::build_as(&body_shape),
                    transform: Transform::from_xyz(0.0, castle_size * 0.25, 0.0),
                    ..default()
                },
                Fill::color(body_color),
            ));

            let tower_width = castle_size / 3.5;
            let tower_height = castle_size / 2.0;

            let tower_shape = shapes::Rectangle {
                extents: Vec2::new(tower_width, tower_height),
                origin: RectangleOrigin::CustomCenter(Vec2::new(0.0, -0.5)),
                ..default()
            };

            parent.spawn((
                CastleTower {
                    city_id: city_name.to_string(),
                    tower_index: 0,
                },
                ShapeBundle {
                    path: GeometryBuilder::build_as(&tower_shape),
                    transform: Transform::from_xyz(-castle_size / 2.0 + tower_width / 2.0, 0.0, 0.1),
                    ..default()
                },
                Fill::color(player_color),
            ));

            let middle_tower_shape = shapes::Rectangle {
                extents: Vec2::new(tower_width, tower_height + castle_size * 0.15),
                origin: RectangleOrigin::CustomCenter(Vec2::new(0.0, -0.5)),
                ..default()
            };

            parent.spawn((
                CastleTower {
                    city_id: city_name.to_string(),
                    tower_index: 1,
                },
                ShapeBundle {
                    path: GeometryBuilder::build_as(&middle_tower_shape),
                    transform: Transform::from_xyz(0.0, -castle_size * 0.15, 0.1),
                    ..default()
                },
                Fill::color(player_color),
            ));

            parent.spawn((
                CastleTower {
                    city_id: city_name.to_string(),
                    tower_index: 2,
                },
                ShapeBundle {
                    path: GeometryBuilder::build_as(&tower_shape),
                    transform: Transform::from_xyz(castle_size / 2.0 - tower_width / 2.0, 0.0, 0.1),
                    ..default()
                },
                Fill::color(player_color),
            ));

            let gate_width = castle_size / 3.0;
            let gate_height = castle_size / 3.0;
            let gate_shape = shapes::Rectangle {
                extents: Vec2::new(gate_width, gate_height),
                origin: RectangleOrigin::CustomCenter(Vec2::new(0.0, -0.5)),
                ..default()
            };

            parent.spawn((
                CastleGate {
                    city_id: city_name.to_string(),
                },
                ShapeBundle {
                    path: GeometryBuilder::build_as(&gate_shape),
                    transform: Transform::from_xyz(0.0, castle_size * 0.5, 0.2),
                    ..default()
                },
                Fill::color(Color::srgba(0.0, 0.0, 0.0, 0.4)),
            ));

            parent.spawn((
                CityTroopText {
                    city_id: city_name.to_string(),
                },
                Text2d::new(format!(
                    "{}/{}/{}",
                    troops.get("A").unwrap_or(&0),
                    troops.get("B").unwrap_or(&0),
                    troops.get("C").unwrap_or(&0)
                )),
                TextFont {
                    font_size: 12.0,
                    ..default()
                },
                TextColor(Color::WHITE),
                Transform::from_xyz(0.0, -castle_size * 0.8, 0.3),
            ));
        });
}

pub fn update_cities(
    state_history: Res<StateHistory>,
    state_buffer: Res<StateBuffer>,
    animation: Res<AnimationState>,
    config: Res<GameConfig>,
    player_registry: Res<PlayerRegistry>,
    mut city_query: Query<(&mut City, &Children)>,
    mut body_query: Query<&mut Fill, (With<CastleBody>, Without<CastleTower>, Without<CastleGate>)>,
    mut tower_query: Query<&mut Fill, (With<CastleTower>, Without<CastleBody>, Without<CastleGate>)>,
    mut text_query: Query<(&CityTroopText, &mut Text2d)>,
) {
    let current_state = get_current_state(&state_history, &state_buffer, &animation, &config);
    let Some(current_state) = current_state else {
        return;
    };

    for (mut city, children) in city_query.iter_mut() {
        if let Some(city_data) = current_state.cities.get(&city.id) {
            city.player = city_data.player.clone();
            city.troops = city_data.troops.clone();

            let player_color = player_registry.get_color(&city.player);
            let body_color = Color::srgba(
                player_color.to_srgba().red * 0.9,
                player_color.to_srgba().green * 0.9,
                player_color.to_srgba().blue * 0.9,
                1.0,
            );

            for child in children.iter() {
                if let Ok(mut fill) = body_query.get_mut(*child) {
                    *fill = Fill::color(body_color);
                }
                if let Ok(mut fill) = tower_query.get_mut(*child) {
                    *fill = Fill::color(player_color);
                }
            }
        }
    }

    for (troop_text, mut text) in text_query.iter_mut() {
        for (city, _) in city_query.iter() {
            if city.id == troop_text.city_id {
                *text = Text2d::new(format!(
                    "{}/{}/{}",
                    city.troops.get("A").unwrap_or(&0),
                    city.troops.get("B").unwrap_or(&0),
                    city.troops.get("C").unwrap_or(&0)
                ));
                break;
            }
        }
    }
}

fn get_current_state<'a>(
    state_history: &'a StateHistory,
    state_buffer: &'a StateBuffer,
    animation: &AnimationState,
    config: &GameConfig,
) -> Option<&'a State> {
    if config.mode == GameMode::Live {
        if animation.display_state_idx >= 0
            && (animation.display_state_idx as usize) < state_buffer.buffer.len()
        {
            Some(&state_buffer.buffer[animation.display_state_idx as usize].state)
        } else {
            None
        }
    } else {
        if animation.current_state_idx < state_history.states.len() {
            Some(&state_history.states[animation.current_state_idx])
        } else {
            None
        }
    }
}
