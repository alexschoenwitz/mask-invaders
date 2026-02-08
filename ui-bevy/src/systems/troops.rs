use bevy::prelude::*;
use bevy_prototype_lyon::prelude::*;

use crate::components::TroopSprite;
use crate::resources::PlayerRegistry;

pub fn draw_troop_sprite(
    builder: &mut ChildBuilder,
    troop_type: &str,
    player: &str,
    player_registry: &PlayerRegistry,
    position: Vec2,
    z_index: f32,
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
            builder.spawn((
                TroopSprite {
                    troop_type: troop_type.to_string(),
                    player: player.to_string(),
                },
                ShapeBundle {
                    path: GeometryBuilder::build_as(&shape),
                    transform: Transform::from_xyz(position.x, position.y, z_index),
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
            builder.spawn((
                TroopSprite {
                    troop_type: troop_type.to_string(),
                    player: player.to_string(),
                },
                ShapeBundle {
                    path: GeometryBuilder::build_as(&shape),
                    transform: Transform::from_xyz(position.x, position.y, z_index),
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

            builder.spawn((
                TroopSprite {
                    troop_type: troop_type.to_string(),
                    player: player.to_string(),
                },
                ShapeBundle {
                    path,
                    transform: Transform::from_xyz(position.x, position.y, z_index),
                    ..default()
                },
                Stroke::new(color, 2.0),
            ));
        }
        _ => {
            let shape = shapes::Circle {
                radius: size / 2.0,
                center: Vec2::ZERO,
            };
            builder.spawn((
                TroopSprite {
                    troop_type: troop_type.to_string(),
                    player: player.to_string(),
                },
                ShapeBundle {
                    path: GeometryBuilder::build_as(&shape),
                    transform: Transform::from_xyz(position.x, position.y, z_index),
                    ..default()
                },
                Fill::color(color),
            ));
        }
    }
}

pub fn spawn_troop_entity(
    commands: &mut Commands,
    troop_type: &str,
    player: &str,
    player_registry: &PlayerRegistry,
    position: Vec3,
) -> Entity {
    let size = 8.0;
    let color = player_registry.get_color(player);

    match troop_type {
        "A" => {
            let shape = shapes::Rectangle {
                extents: Vec2::new(size, size),
                origin: RectangleOrigin::Center,
                ..default()
            };
            commands
                .spawn((
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
                ))
                .id()
        }
        "B" => {
            let shape = shapes::Circle {
                radius: size / 2.0,
                center: Vec2::ZERO,
            };
            commands
                .spawn((
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
                ))
                .id()
        }
        "C" => {
            let mut path_builder = PathBuilder::new();
            path_builder.move_to(Vec2::new(0.0, -size / 2.0));
            path_builder.line_to(Vec2::new(-size / 2.0, size / 2.0));
            path_builder.line_to(Vec2::new(size / 2.0, size / 2.0));
            path_builder.close();
            let path = path_builder.build();

            commands
                .spawn((
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
                ))
                .id()
        }
        _ => {
            let shape = shapes::Circle {
                radius: size / 2.0,
                center: Vec2::ZERO,
            };
            commands
                .spawn((
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
                ))
                .id()
        }
    }
}
