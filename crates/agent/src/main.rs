//! Aula agent: student-side binary.

use anyhow::Result;

fn main() -> Result<()> {
    tracing_subscriber::fmt::init();
    tracing::info!("Aula agent starting");
    Ok(())
}

#[cfg(test)]
mod tests {
    #[test]
    fn agent_crate_loads() {
        assert_eq!(2 + 2, 4);
    }
}
