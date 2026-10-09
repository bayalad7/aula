//! Aula console entry point.

#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

fn main() {
    aula_console_lib::run();
}
