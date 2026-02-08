use std::env;
use std::path::PathBuf;
use std::process::Command;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let out_dir = PathBuf::from(env::var("OUT_DIR")?);
    let proto_export_dir = out_dir.join("proto");

    // Use buf to export proto files with dependencies
    let status = Command::new("buf")
        .args(["export", "..", "--output"])
        .arg(&proto_export_dir)
        .status()?;

    if !status.success() {
        return Err("buf export failed".into());
    }

    let descriptor_path = out_dir.join("api_descriptor.bin");

    let proto_file = proto_export_dir.join("server/api/api.proto");

    prost_build::Config::new()
        .file_descriptor_set_path(&descriptor_path)
        .compile_protos(&[&proto_file], &[&proto_export_dir])?;

    let descriptor_set = std::fs::read(&descriptor_path)?;

    pbjson_build::Builder::new()
        .register_descriptors(&descriptor_set)?
        .build(&[".api"])?;

    println!("cargo:rerun-if-changed=../server/api/api.proto");

    Ok(())
}
