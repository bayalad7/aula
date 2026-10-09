//! Aula platform: OS-specific trait implementations.
//!
//! Provides platform abstractions behind `#[cfg(target_os = ...)]` gates.

#[cfg(target_os = "windows")]
pub mod windows;

#[cfg(target_os = "macos")]
pub mod macos;

#[cfg(target_os = "linux")]
pub mod linux;

#[cfg(not(any(target_os = "windows", target_os = "macos", target_os = "linux")))]
pub mod unsupported;

#[cfg(test)]
mod tests {
    #[test]
    fn platform_crate_loads() {
        assert_eq!(2 + 2, 4);
    }
}
