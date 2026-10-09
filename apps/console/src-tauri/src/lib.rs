//! Aula console: teacher's desktop application (Tauri 2).

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .run(tauri::generate_context!())
        .expect("error while running Aula console");
}

#[cfg(test)]
mod tests {
    #[test]
    fn console_crate_loads() {
        assert_eq!(2 + 2, 4);
    }
}
