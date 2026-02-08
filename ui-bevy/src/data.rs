pub mod api {
    include!(concat!(env!("OUT_DIR"), "/api.rs"));
    include!(concat!(env!("OUT_DIR"), "/api.serde.rs"));
}

pub use api::*;
