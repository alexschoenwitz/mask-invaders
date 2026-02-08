use bevy::prelude::*;
use bevy_prototype_lyon::prelude::*;

use crate::components::{Ground, Moon, Sky, Sun};
use crate::resources::ScreenDimensions;

pub fn setup_camera(mut commands: Commands) {
    commands.spawn(Camera2d::default());
}

pub fn setup_background(mut commands: Commands, screen: Res<ScreenDimensions>) {
    let sky_height = screen.height / 6.0;

    let sky_shape = shapes::Rectangle {
        extents: Vec2::new(screen.width, sky_height),
        origin: RectangleOrigin::TopLeft,
        ..default()
    };

    commands.spawn((
        Sky,
        ShapeBundle {
            path: GeometryBuilder::build_as(&sky_shape),
            transform: Transform::from_xyz(-screen.width / 2.0, screen.height / 2.0, 0.0),
            ..default()
        },
        Fill::color(Color::srgba(135.0 / 255.0, 206.0 / 255.0, 235.0 / 255.0, 1.0)),
    ));

    let ground_shape = shapes::Rectangle {
        extents: Vec2::new(screen.width, screen.height - sky_height),
        origin: RectangleOrigin::TopLeft,
        ..default()
    };

    commands.spawn((
        Ground,
        ShapeBundle {
            path: GeometryBuilder::build_as(&ground_shape),
            transform: Transform::from_xyz(
                -screen.width / 2.0,
                screen.height / 2.0 - sky_height,
                0.0,
            ),
            ..default()
        },
        Fill::color(Color::srgba(34.0 / 255.0, 139.0 / 255.0, 34.0 / 255.0, 1.0)),
    ));

    let sun_shape = shapes::Circle {
        radius: 12.0,
        center: Vec2::ZERO,
    };

    commands.spawn((
        Sun,
        ShapeBundle {
            path: GeometryBuilder::build_as(&sun_shape),
            transform: Transform::from_xyz(0.0, screen.height / 2.0 - 50.0, 1.0),
            visibility: Visibility::Hidden,
            ..default()
        },
        Fill::color(Color::srgba(1.0, 220.0 / 255.0, 0.0, 1.0)),
    ));

    let moon_shape = shapes::Circle {
        radius: 10.0,
        center: Vec2::ZERO,
    };

    commands.spawn((
        Moon,
        ShapeBundle {
            path: GeometryBuilder::build_as(&moon_shape),
            transform: Transform::from_xyz(0.0, screen.height / 2.0 - 50.0, 1.0),
            visibility: Visibility::Hidden,
            ..default()
        },
        Fill::color(Color::srgba(220.0 / 255.0, 220.0 / 255.0, 240.0 / 255.0, 1.0)),
    ));
}
