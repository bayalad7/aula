//! Aula core: networking, discovery, TLS, authentication, and session management.
//!
//! This crate is OS-free by design — all OS-specific functionality is accessed
//! through traits defined in `aula-platform`.

pub mod auth;
pub mod client;
pub mod discovery;
pub mod error;
pub mod server;
pub mod tls;

pub use aula_protocol;
