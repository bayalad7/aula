//! Aula core: networking, discovery, TLS, authentication, and session management.
//!
//! This crate is OS-free by design — all OS-specific functionality is accessed
//! through traits defined in `aula-platform`.

pub use aula_protocol;

#[cfg(test)]
mod tests {
    #[test]
    fn core_crate_loads() {
        assert_eq!(2 + 2, 4);
    }
}
