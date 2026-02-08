use bevy::prelude::*;
use bevy_prototype_lyon::prelude::*;

use crate::components::{Moon, Sky, Sun};
use crate::resources::{AnimationState, ScreenDimensions};

pub fn update_day_night_cycle(
    animation: Res<AnimationState>,
    mut sky_query: Query<&mut Fill, With<Sky>>,
) {
    let time_of_day = (animation.current_turn % 100.0) / 100.0;

    let (sky_r, sky_g, sky_b) = calculate_sky_color(time_of_day);
    let sky_color = Color::srgba(sky_r, sky_g, sky_b, 1.0);

    for mut fill in sky_query.iter_mut() {
        *fill = Fill::color(sky_color);
    }
}

fn calculate_sky_color(time_of_day: f64) -> (f32, f32, f32) {
    if time_of_day < 0.2 {
        (25.0 / 255.0, 25.0 / 255.0, 50.0 / 255.0)
    } else if time_of_day < 0.35 {
        let t = (time_of_day - 0.2) / 0.15;
        if t < 0.5 {
            let t2 = t * 2.0;
            (
                (25.0 + 230.0 * t2) as f32 / 255.0,
                (25.0 + 115.0 * t2) as f32 / 255.0,
                (50.0 + 30.0 * t2) as f32 / 255.0,
            )
        } else {
            let t2 = (t - 0.5) * 2.0;
            (
                (255.0 - 120.0 * t2) as f32 / 255.0,
                (140.0 + 66.0 * t2) as f32 / 255.0,
                (80.0 + 155.0 * t2) as f32 / 255.0,
            )
        }
    } else if time_of_day < 0.65 {
        (135.0 / 255.0, 206.0 / 255.0, 235.0 / 255.0)
    } else if time_of_day < 0.8 {
        let t = (time_of_day - 0.65) / 0.15;
        if t < 0.5 {
            let t2 = t * 2.0;
            (
                (135.0 + 120.0 * t2) as f32 / 255.0,
                (206.0 - 66.0 * t2) as f32 / 255.0,
                (235.0 - 155.0 * t2) as f32 / 255.0,
            )
        } else {
            let t2 = (t - 0.5) * 2.0;
            (
                (255.0 - 230.0 * t2) as f32 / 255.0,
                (140.0 - 115.0 * t2) as f32 / 255.0,
                (80.0 - 30.0 * t2) as f32 / 255.0,
            )
        }
    } else {
        (25.0 / 255.0, 25.0 / 255.0, 50.0 / 255.0)
    }
}

pub fn update_celestial_bodies(
    animation: Res<AnimationState>,
    screen: Res<ScreenDimensions>,
    mut sun_query: Query<(&mut Transform, &mut Visibility), (With<Sun>, Without<Moon>)>,
    mut moon_query: Query<(&mut Transform, &mut Visibility), (With<Moon>, Without<Sun>)>,
) {
    let time_of_day = (animation.current_turn % 100.0) / 100.0;
    let sky_width = screen.width;
    let sky_height = screen.height / 6.0;

    for (mut transform, mut visibility) in sun_query.iter_mut() {
        if time_of_day >= 0.2 && time_of_day <= 0.8 {
            *visibility = Visibility::Visible;
            let sun_progress = (time_of_day - 0.2) / 0.6;
            let sun_x = -sky_width / 2.0 + sky_width * 0.1 + sky_width * 0.8 * sun_progress as f32;
            let sun_y = screen.height / 2.0 - sky_height * 0.9
                + (sun_progress * std::f64::consts::PI).sin() as f32 * sky_height * 0.7;
            transform.translation = Vec3::new(sun_x, sun_y, 1.0);
        } else {
            *visibility = Visibility::Hidden;
        }
    }

    for (mut transform, mut visibility) in moon_query.iter_mut() {
        if time_of_day >= 0.8 || time_of_day <= 0.2 {
            *visibility = Visibility::Visible;
            let moon_progress = if time_of_day >= 0.8 {
                (time_of_day - 0.8) / 0.4
            } else {
                (time_of_day + 0.2) / 0.4
            };
            let moon_x =
                -sky_width / 2.0 + sky_width * 0.1 + sky_width * 0.8 * moon_progress as f32;
            let moon_y = screen.height / 2.0 - sky_height * 0.9
                + (moon_progress * std::f64::consts::PI).sin() as f32 * sky_height * 0.7;
            transform.translation = Vec3::new(moon_x, moon_y, 1.0);
        } else {
            *visibility = Visibility::Hidden;
        }
    }
}
