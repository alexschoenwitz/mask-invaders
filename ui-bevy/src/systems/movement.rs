use bevy::prelude::*;
use bevy_prototype_lyon::prelude::*;

use crate::components::{Movement, MovementTroop, TroopSprite};
use crate::data::{movement, State};
use crate::resources::{
    AnimationState, CityPositions, GameConfig, GameMode, MovementStartCache, PlayerRegistry,
    StateBuffer, StateHistory,
};

pub fn update_movements(
    mut commands: Commands,
    state_history: Res<StateHistory>,
    state_buffer: Res<StateBuffer>,
    animation: Res<AnimationState>,
    config: Res<GameConfig>,
    city_positions: Res<CityPositions>,
    player_registry: Res<PlayerRegistry>,
    mut movement_cache: ResMut<MovementStartCache>,
    existing_movements: Query<Entity, With<Movement>>,
    existing_troops: Query<Entity, With<MovementTroop>>,
) {
    let current_state = get_current_state(&state_history, &state_buffer, &animation, &config);
    let Some(current_state) = current_state else {
        return;
    };

    for entity in existing_movements.iter() {
        commands.entity(entity).despawn_recursive();
    }
    for entity in existing_troops.iter() {
        commands.entity(entity).despawn_recursive();
    }

    let current_time = animation.current_turn;

    for movement_data in &current_state.movements {
        let to_city = match &movement_data.to {
            Some(movement::To::City(city)) => city,
            _ => continue,
        };

        let Some(from_pos) = city_positions.positions.get(&movement_data.from) else {
            continue;
        };
        let Some(to_pos) = city_positions.positions.get(to_city.as_str()) else {
            continue;
        };

        let movement_id = format!(
            "{}->{}@{}:{}",
            movement_data.from, to_city, movement_data.arriving_turn, movement_data.player
        );

        let start_turn = *movement_cache
            .cache
            .entry(movement_id.clone())
            .or_insert(current_state.turn);

        let total_duration = (movement_data.arriving_turn - start_turn) as f64;
        let progress = if total_duration > 0.0 {
            let elapsed = current_time - start_turn as f64;
            (elapsed / total_duration).clamp(0.0, 1.0)
        } else {
            0.0
        };

        if progress < 0.0 || progress > 1.0 {
            continue;
        }

        let current_x = from_pos.x + (to_pos.x - from_pos.x) * progress as f32;
        let current_y = from_pos.y + (to_pos.y - from_pos.y) * progress as f32;

        commands.spawn((
            Movement {
                from_city: movement_data.from.clone(),
                to_city: to_city.to_string(),
                troops: movement_data.troops.clone(),
                player: movement_data.player.clone(),
                arriving_turn: movement_data.arriving_turn,
                start_turn,
                progress,
            },
            Transform::from_xyz(current_x, current_y, 10.0),
            Visibility::Visible,
        ));

        let troop_types = ["A", "B", "C"];
        for (i, troop_type) in troop_types.iter().enumerate() {
            let count = movement_data.troops.get(*troop_type).unwrap_or(&0);
            if *count > 0 {
                let offset_x = current_x + (i as i32 - 1) as f32 * 8.0;
                let offset_y = current_y + (i as i32 - 1) as f32 * 8.0;

                spawn_movement_troop(
                    &mut commands,
                    troop_type,
                    &movement_data.player,
                    &player_registry,
                    Vec3::new(offset_x, offset_y, 10.0 + i as f32 * 0.1),
                    &movement_id,
                    i as i32,
                );
            }
        }
    }
}

fn spawn_movement_troop(
    commands: &mut Commands,
    troop_type: &str,
    player: &str,
    player_registry: &PlayerRegistry,
    position: Vec3,
    movement_id: &str,
    offset_index: i32,
) {
    let size = 8.0;
    let color = player_registry.get_color(player);

    match troop_type {
        "A" => {
            let shape = shapes::Rectangle {
                extents: Vec2::new(size, size),
                origin: RectangleOrigin::Center,
                ..default()
            };
            commands.spawn((
                MovementTroop {
                    movement_id: movement_id.to_string(),
                    troop_type: troop_type.to_string(),
                    offset_index,
                },
                TroopSprite {
                    troop_type: troop_type.to_string(),
                    player: player.to_string(),
                },
                ShapeBundle {
                    path: GeometryBuilder::build_as(&shape),
                    transform: Transform::from_translation(position),
                    ..default()
                },
                Fill::color(color),
            ));
        }
        "B" => {
            let shape = shapes::Circle {
                radius: size / 2.0,
                center: Vec2::ZERO,
            };
            commands.spawn((
                MovementTroop {
                    movement_id: movement_id.to_string(),
                    troop_type: troop_type.to_string(),
                    offset_index,
                },
                TroopSprite {
                    troop_type: troop_type.to_string(),
                    player: player.to_string(),
                },
                ShapeBundle {
                    path: GeometryBuilder::build_as(&shape),
                    transform: Transform::from_translation(position),
                    ..default()
                },
                Fill::color(color),
            ));
        }
        "C" => {
            let mut path_builder = PathBuilder::new();
            path_builder.move_to(Vec2::new(0.0, -size / 2.0));
            path_builder.line_to(Vec2::new(-size / 2.0, size / 2.0));
            path_builder.line_to(Vec2::new(size / 2.0, size / 2.0));
            path_builder.close();
            let path = path_builder.build();

            commands.spawn((
                MovementTroop {
                    movement_id: movement_id.to_string(),
                    troop_type: troop_type.to_string(),
                    offset_index,
                },
                TroopSprite {
                    troop_type: troop_type.to_string(),
                    player: player.to_string(),
                },
                ShapeBundle {
                    path,
                    transform: Transform::from_translation(position),
                    ..default()
                },
                Stroke::new(color, 2.0),
            ));
        }
        _ => {}
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
