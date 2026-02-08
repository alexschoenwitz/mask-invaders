use bevy::prelude::*;
use bevy_prototype_lyon::prelude::*;

use crate::components::{DropdownHeader, DropdownOption, TurnCounter};
use crate::resources::{
    AnimationState, DropdownState, GameConfig, GameMode, ScreenDimensions, StateBuffer,
};

#[derive(Component)]
pub struct DropdownBackground;

#[derive(Component)]
pub struct DropdownArrow;

#[derive(Component)]
pub struct DropdownOptionsBackground;

pub fn setup_ui(mut commands: Commands, screen: Res<ScreenDimensions>) {
    commands.spawn((
        TurnCounter,
        Text2d::new("Turn: 0"),
        TextFont {
            font_size: 14.0,
            ..default()
        },
        TextColor(Color::WHITE),
        Transform::from_xyz(-screen.width / 2.0 + 10.0, screen.height / 2.0 - 10.0, 100.0),
    ));
}

pub fn update_turn_counter(
    config: Res<GameConfig>,
    animation: Res<AnimationState>,
    state_buffer: Res<StateBuffer>,
    mut query: Query<&mut Text2d, With<TurnCounter>>,
) {
    let mode_str = match config.mode {
        GameMode::Live => "Live",
        GameMode::Replay => "Replay",
    };

    for mut text in query.iter_mut() {
        if config.mode == GameMode::Live {
            if animation.display_state_idx >= 0
                && (animation.display_state_idx as usize) < state_buffer.buffer.len()
            {
                let turn = state_buffer.buffer[animation.display_state_idx as usize].state.turn;
                let buffer_info = format!(
                    " (buffer: {}/{}, progress: {:.0}%)",
                    animation.display_state_idx + 1,
                    state_buffer.buffer.len(),
                    animation.turn_progress * 100.0
                );
                *text = Text2d::new(format!("Turn: {} [{}]{}", turn, mode_str, buffer_info));
            } else {
                let buffered = state_buffer.buffer.len();
                let needed = StateBuffer::MIN_BUFFER_STATES;
                *text = Text2d::new(format!(
                    "Buffering... [{}] ({}/{} states)",
                    mode_str, buffered, needed
                ));
            }
        } else {
            *text = Text2d::new(format!(
                "Turn: {:.0} [{}]",
                animation.current_turn, mode_str
            ));
        }
    }
}

pub fn setup_dropdown(mut commands: Commands, screen: Res<ScreenDimensions>) {
    let dropdown_x = -screen.width / 2.0 + 10.0;
    let dropdown_y = screen.height / 2.0 - 45.0;
    let dropdown_width = 200.0;
    let dropdown_height = 25.0;

    let bg_shape = shapes::Rectangle {
        extents: Vec2::new(dropdown_width, dropdown_height),
        origin: RectangleOrigin::TopLeft,
        ..default()
    };

    commands.spawn((
        DropdownBackground,
        ShapeBundle {
            path: GeometryBuilder::build_as(&bg_shape),
            transform: Transform::from_xyz(dropdown_x, dropdown_y, 100.0),
            ..default()
        },
        Fill::color(Color::srgba(40.0 / 255.0, 40.0 / 255.0, 50.0 / 255.0, 0.9)),
        Stroke::new(
            Color::srgba(100.0 / 255.0, 100.0 / 255.0, 120.0 / 255.0, 1.0),
            1.0,
        ),
    ));

    commands.spawn((
        DropdownHeader,
        Text2d::new("No game"),
        TextFont {
            font_size: 12.0,
            ..default()
        },
        TextColor(Color::WHITE),
        Transform::from_xyz(dropdown_x + 5.0, dropdown_y - dropdown_height / 2.0, 101.0),
    ));

    commands.spawn((
        DropdownArrow,
        Text2d::new("v"),
        TextFont {
            font_size: 12.0,
            ..default()
        },
        TextColor(Color::WHITE),
        Transform::from_xyz(
            dropdown_x + dropdown_width - 15.0,
            dropdown_y - dropdown_height / 2.0,
            101.0,
        ),
    ));
}

pub fn update_dropdown(
    config: Res<GameConfig>,
    dropdown: Res<DropdownState>,
    mut header_query: Query<&mut Text2d, (With<DropdownHeader>, Without<DropdownArrow>)>,
    mut arrow_query: Query<&mut Text2d, (With<DropdownArrow>, Without<DropdownHeader>)>,
    mut bg_query: Query<&mut Visibility, With<DropdownBackground>>,
) {
    if config.mode != GameMode::Live {
        for mut vis in bg_query.iter_mut() {
            *vis = Visibility::Hidden;
        }
        return;
    }

    for mut vis in bg_query.iter_mut() {
        *vis = Visibility::Visible;
    }

    for mut text in header_query.iter_mut() {
        let game_text = dropdown
            .selected_game
            .as_ref()
            .map(|s| {
                if s.len() > 25 {
                    format!("{}...", &s[..22])
                } else {
                    s.clone()
                }
            })
            .unwrap_or_else(|| "No game".to_string());
        *text = Text2d::new(game_text);
    }

    for mut text in arrow_query.iter_mut() {
        *text = Text2d::new(if dropdown.expanded { "^" } else { "v" });
    }
}

pub fn render_dropdown_options(
    mut commands: Commands,
    config: Res<GameConfig>,
    dropdown: Res<DropdownState>,
    screen: Res<ScreenDimensions>,
    existing_options: Query<Entity, With<DropdownOption>>,
    existing_options_bg: Query<Entity, With<DropdownOptionsBackground>>,
) {
    for entity in existing_options.iter() {
        commands.entity(entity).despawn();
    }
    for entity in existing_options_bg.iter() {
        commands.entity(entity).despawn();
    }

    if config.mode != GameMode::Live || !dropdown.expanded || dropdown.available_games.is_empty() {
        return;
    }

    let dropdown_x = -screen.width / 2.0 + 10.0;
    let dropdown_y = screen.height / 2.0 - 45.0;
    let dropdown_width = 200.0;
    let dropdown_height = 25.0;
    let option_height = 20.0;
    let total_height = dropdown.available_games.len() as f32 * option_height;

    let options_bg = shapes::Rectangle {
        extents: Vec2::new(dropdown_width, total_height),
        origin: RectangleOrigin::TopLeft,
        ..default()
    };

    commands.spawn((
        DropdownOptionsBackground,
        ShapeBundle {
            path: GeometryBuilder::build_as(&options_bg),
            transform: Transform::from_xyz(dropdown_x, dropdown_y - dropdown_height, 100.0),
            ..default()
        },
        Fill::color(Color::srgba(30.0 / 255.0, 30.0 / 255.0, 40.0 / 255.0, 0.94)),
        Stroke::new(
            Color::srgba(100.0 / 255.0, 100.0 / 255.0, 120.0 / 255.0, 1.0),
            1.0,
        ),
    ));

    for (i, game_id) in dropdown.available_games.iter().enumerate() {
        let option_y = dropdown_y - dropdown_height - (i as f32) * option_height;

        if dropdown.selected_game.as_ref() == Some(game_id) {
            let highlight = shapes::Rectangle {
                extents: Vec2::new(dropdown_width - 2.0, option_height - 1.0),
                origin: RectangleOrigin::TopLeft,
                ..default()
            };
            commands.spawn((
                DropdownOption {
                    game_id: game_id.clone(),
                },
                ShapeBundle {
                    path: GeometryBuilder::build_as(&highlight),
                    transform: Transform::from_xyz(dropdown_x + 1.0, option_y - 0.5, 100.5),
                    ..default()
                },
                Fill::color(Color::srgba(60.0 / 255.0, 60.0 / 255.0, 80.0 / 255.0, 1.0)),
            ));
        }

        let display_text = if game_id.len() > 25 {
            format!("{}...", &game_id[..22])
        } else {
            game_id.clone()
        };

        commands.spawn((
            DropdownOption {
                game_id: game_id.clone(),
            },
            Text2d::new(display_text),
            TextFont {
                font_size: 11.0,
                ..default()
            },
            TextColor(Color::WHITE),
            Transform::from_xyz(dropdown_x + 5.0, option_y - option_height / 2.0, 101.0),
        ));
    }
}
