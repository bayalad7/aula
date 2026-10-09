//! Agent capabilities declared during the hello handshake.

use serde::{Deserialize, Serialize};

/// A feature the agent supports on its platform.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum Capability {
    #[serde(rename = "code.view")]
    CodeView,
    #[serde(rename = "msg.show")]
    MsgShow,
    #[serde(rename = "screen.thumb")]
    ScreenThumb,
    #[serde(rename = "screen.lock")]
    ScreenLock,
    #[serde(rename = "screen.unlock")]
    ScreenUnlock,
    #[serde(rename = "input.block")]
    InputBlock,
    #[serde(rename = "code.collect")]
    CodeCollect,
    #[serde(rename = "file.collect")]
    FileCollect,
    #[serde(rename = "app.list")]
    AppList,
}
