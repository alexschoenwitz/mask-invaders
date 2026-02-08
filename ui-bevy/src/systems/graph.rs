use bevy::prelude::*;
use bevy_prototype_lyon::prelude::*;

use crate::components::StatsGraph;
use crate::resources::{PlayerRegistry, ScreenDimensions, StateHistory};

#[derive(Component)]
pub struct GraphBackground;

#[derive(Component)]
pub struct GraphAxis;

#[derive(Component)]
pub struct GraphGridLine;

#[derive(Component)]
pub struct GraphPlayerLine {
    pub player: String,
}

#[derive(Component)]
pub struct GraphLegendEntry {
    pub player: String,
}

#[derive(Component)]
pub struct GraphTitle;

#[derive(Component)]
pub struct GraphAxisLabel;

pub fn setup_graph(mut commands: Commands, screen: Res<ScreenDimensions>) {
    let graph_height = 200.0;
    let graph_width = screen.width - 20.0;
    let graph_x = -screen.width / 2.0 + 10.0;
    let graph_y = -screen.height / 2.0 + 10.0;

    let bg_shape = shapes::Rectangle {
        extents: Vec2::new(graph_width, graph_height),
        origin: RectangleOrigin::BottomLeft,
        ..default()
    };

    commands.spawn((
        StatsGraph,
        GraphBackground,
        ShapeBundle {
            path: GeometryBuilder::build_as(&bg_shape),
            transform: Transform::from_xyz(graph_x, graph_y, 50.0),
            ..default()
        },
        Fill::color(Color::srgba(0.0, 0.0, 0.0, 0.78)),
    ));
}

pub fn update_graph(
    mut commands: Commands,
    state_history: Res<StateHistory>,
    player_registry: Res<PlayerRegistry>,
    screen: Res<ScreenDimensions>,
    graph_lines: Query<Entity, With<GraphPlayerLine>>,
    legend_entries: Query<Entity, With<GraphLegendEntry>>,
    grid_lines: Query<Entity, With<GraphGridLine>>,
    axis_query: Query<Entity, With<GraphAxis>>,
    title_query: Query<Entity, With<GraphTitle>>,
    label_query: Query<Entity, With<GraphAxisLabel>>,
) {
    if state_history.states.len() < 2 {
        return;
    }

    for entity in graph_lines.iter() {
        commands.entity(entity).despawn();
    }
    for entity in legend_entries.iter() {
        commands.entity(entity).despawn();
    }
    for entity in grid_lines.iter() {
        commands.entity(entity).despawn();
    }
    for entity in axis_query.iter() {
        commands.entity(entity).despawn();
    }
    for entity in title_query.iter() {
        commands.entity(entity).despawn();
    }
    for entity in label_query.iter() {
        commands.entity(entity).despawn();
    }

    let graph_height = 200.0;
    let graph_width = screen.width - 20.0;
    let graph_x = -screen.width / 2.0 + 10.0;
    let graph_y = -screen.height / 2.0 + 10.0;

    let plot_x = graph_x + 50.0;
    let plot_y = graph_y + 20.0;
    let plot_width = graph_width - 70.0;
    let plot_height = graph_height - 50.0;

    let mut history: std::collections::HashMap<String, Vec<(i64, i64)>> =
        std::collections::HashMap::new();

    for player in &player_registry.player_list {
        history.insert(player.clone(), Vec::new());
    }

    for state in &state_history.states {
        let mut player_troops: std::collections::HashMap<String, i64> =
            std::collections::HashMap::new();

        for city in state.cities.values() {
            let total = city.troops.get("A").unwrap_or(&0)
                + city.troops.get("B").unwrap_or(&0)
                + city.troops.get("C").unwrap_or(&0);
            *player_troops.entry(city.player.clone()).or_insert(0) += total;
        }

        for movement in &state.movements {
            let total = movement.troops.get("A").unwrap_or(&0)
                + movement.troops.get("B").unwrap_or(&0)
                + movement.troops.get("C").unwrap_or(&0);
            *player_troops.entry(movement.player.clone()).or_insert(0) += total;
        }

        for player in &player_registry.player_list {
            let troops = *player_troops.get(player).unwrap_or(&0);
            if let Some(h) = history.get_mut(player) {
                h.push((state.turn, troops));
            }
        }
    }

    let mut max_units: i64 = 1;
    let mut max_turn: i64 = 1;
    for h in history.values() {
        for (turn, units) in h {
            if *units > max_units {
                max_units = *units;
            }
            if *turn > max_turn {
                max_turn = *turn;
            }
        }
    }

    commands.spawn((
        GraphTitle,
        Text2d::new("=== TROOPS OVER TIME ==="),
        TextFont {
            font_size: 12.0,
            ..default()
        },
        TextColor(Color::WHITE),
        Transform::from_xyz(graph_x + 10.0, graph_y + graph_height - 15.0, 51.0),
    ));

    let grid_color = Color::srgba(60.0 / 255.0, 60.0 / 255.0, 70.0 / 255.0, 1.0);
    for i in 0..=5 {
        let y = plot_y + (i as f32) * plot_height / 5.0;

        let mut path_builder = PathBuilder::new();
        path_builder.move_to(Vec2::new(plot_x, y));
        path_builder.line_to(Vec2::new(plot_x + plot_width, y));
        let path = path_builder.build();

        commands.spawn((
            GraphGridLine,
            ShapeBundle {
                path,
                transform: Transform::from_xyz(0.0, 0.0, 51.0),
                ..default()
            },
            Stroke::new(grid_color, 1.0),
        ));

        let label_value = max_units * (5 - i) as i64 / 5;
        commands.spawn((
            GraphAxisLabel,
            Text2d::new(format!("{}", label_value)),
            TextFont {
                font_size: 10.0,
                ..default()
            },
            TextColor(Color::WHITE),
            Transform::from_xyz(graph_x + 5.0, y, 51.0),
        ));
    }

    let axis_color = Color::srgba(150.0 / 255.0, 150.0 / 255.0, 150.0 / 255.0, 1.0);

    let mut y_axis_builder = PathBuilder::new();
    y_axis_builder.move_to(Vec2::new(plot_x, plot_y));
    y_axis_builder.line_to(Vec2::new(plot_x, plot_y + plot_height));
    commands.spawn((
        GraphAxis,
        ShapeBundle {
            path: y_axis_builder.build(),
            transform: Transform::from_xyz(0.0, 0.0, 51.0),
            ..default()
        },
        Stroke::new(axis_color, 2.0),
    ));

    let mut x_axis_builder = PathBuilder::new();
    x_axis_builder.move_to(Vec2::new(plot_x, plot_y));
    x_axis_builder.line_to(Vec2::new(plot_x + plot_width, plot_y));
    commands.spawn((
        GraphAxis,
        ShapeBundle {
            path: x_axis_builder.build(),
            transform: Transform::from_xyz(0.0, 0.0, 51.0),
            ..default()
        },
        Stroke::new(axis_color, 2.0),
    ));

    for player in &player_registry.player_list {
        let Some(player_history) = history.get(player) else {
            continue;
        };
        if player_history.len() < 2 {
            continue;
        }

        let player_color = player_registry.get_color(player);

        let mut path_builder = PathBuilder::new();
        let mut first = true;

        for (turn, units) in player_history {
            let x = plot_x + (*turn as f32 / max_turn as f32) * plot_width;
            let y = plot_y + (*units as f32 / max_units as f32) * plot_height;

            if first {
                path_builder.move_to(Vec2::new(x, y));
                first = false;
            } else {
                path_builder.line_to(Vec2::new(x, y));
            }
        }

        commands.spawn((
            GraphPlayerLine {
                player: player.clone(),
            },
            ShapeBundle {
                path: path_builder.build(),
                transform: Transform::from_xyz(0.0, 0.0, 52.0),
                ..default()
            },
            Stroke::new(player_color, 2.0),
        ));

        if let Some((last_turn, last_units)) = player_history.last() {
            let x = plot_x + (*last_turn as f32 / max_turn as f32) * plot_width;
            let y = plot_y + (*last_units as f32 / max_units as f32) * plot_height;

            let circle = shapes::Circle {
                radius: 4.0,
                center: Vec2::ZERO,
            };
            commands.spawn((
                GraphPlayerLine {
                    player: player.clone(),
                },
                ShapeBundle {
                    path: GeometryBuilder::build_as(&circle),
                    transform: Transform::from_xyz(x, y, 53.0),
                    ..default()
                },
                Fill::color(player_color),
            ));
        }
    }

    let legend_x = graph_x + graph_width - 150.0;
    let legend_y = graph_y + graph_height - 35.0;

    for (i, player) in player_registry.player_list.iter().enumerate() {
        let player_color = player_registry.get_color(player);
        let y_offset = legend_y - (i as f32) * 15.0;

        let color_box = shapes::Rectangle {
            extents: Vec2::new(10.0, 10.0),
            origin: RectangleOrigin::BottomLeft,
            ..default()
        };
        commands.spawn((
            GraphLegendEntry {
                player: player.clone(),
            },
            ShapeBundle {
                path: GeometryBuilder::build_as(&color_box),
                transform: Transform::from_xyz(legend_x, y_offset, 51.0),
                ..default()
            },
            Fill::color(player_color),
        ));

        let display_name = player_registry.get_display_name(player);
        let current_units = history
            .get(player)
            .and_then(|h| h.last())
            .map(|(_, u)| *u)
            .unwrap_or(0);
        let current_castles = state_history
            .states
            .last()
            .map(|s| {
                s.cities
                    .values()
                    .filter(|c| &c.player == player)
                    .count()
            })
            .unwrap_or(0);

        commands.spawn((
            GraphLegendEntry {
                player: player.clone(),
            },
            Text2d::new(format!(
                "{}: {} ({})",
                display_name, current_units, current_castles
            )),
            TextFont {
                font_size: 10.0,
                ..default()
            },
            TextColor(Color::WHITE),
            Transform::from_xyz(legend_x + 15.0, y_offset + 5.0, 51.0),
        ));
    }

    commands.spawn((
        GraphAxisLabel,
        Text2d::new("Turn"),
        TextFont {
            font_size: 10.0,
            ..default()
        },
        TextColor(Color::WHITE),
        Transform::from_xyz(plot_x + plot_width / 2.0, plot_y - 10.0, 51.0),
    ));

    commands.spawn((
        GraphAxisLabel,
        Text2d::new("Units"),
        TextFont {
            font_size: 10.0,
            ..default()
        },
        TextColor(Color::WHITE),
        Transform::from_xyz(graph_x + 5.0, plot_y + plot_height + 10.0, 51.0),
    ));
}
