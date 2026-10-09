//! Integration tests for aula-protocol.

use aula_protocol::*;
use uuid::Uuid;

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

fn sample_uuid() -> Uuid {
    Uuid::parse_str("550e8400-e29b-41d4-a716-446655440000").unwrap()
}

fn ts() -> u64 {
    1_728_400_000_000
}

// ---------------------------------------------------------------------------
// Round-trip tests: encode → decode → compare
// ---------------------------------------------------------------------------

#[test]
fn round_trip_hello() {
    let msg = Message::Hello(HelloPayload {
        hostname: "PC-LAB-12".into(),
        username: "alumno".into(),
        os: "windows".into(),
        os_version: "10.0.19045".into(),
        agent_version: "0.1.0".into(),
        capabilities: vec![Capability::CodeView, Capability::MsgShow],
    });
    let env = Envelope::new(ts(), msg);
    let json = encode(&env).unwrap();
    let decoded = decode(&json).unwrap();
    assert_eq!(env, decoded);
}

#[test]
fn round_trip_auth_challenge() {
    let msg = Message::AuthChallenge(AuthChallengePayload {
        ref_id: sample_uuid(),
        nonce: "dGhpcyBpcyBhIHRlc3Q=".into(),
    });
    let env = Envelope::new(ts(), msg);
    let json = encode(&env).unwrap();
    let decoded = decode(&json).unwrap();
    assert_eq!(env, decoded);
}

#[test]
fn round_trip_auth_proof() {
    let msg = Message::AuthProof(AuthProofPayload {
        ref_id: sample_uuid(),
        signature: "c2lnbmF0dXJl".into(),
    });
    let env = Envelope::new(ts(), msg);
    let json = encode(&env).unwrap();
    let decoded = decode(&json).unwrap();
    assert_eq!(env, decoded);
}

#[test]
fn round_trip_auth_ok() {
    let msg = Message::AuthOk(AuthOkPayload {
        ref_id: sample_uuid(),
    });
    let env = Envelope::new(ts(), msg);
    let json = encode(&env).unwrap();
    let decoded = decode(&json).unwrap();
    assert_eq!(env, decoded);
}

#[test]
fn round_trip_heartbeat() {
    let msg = Message::Heartbeat(HeartbeatPayload {});
    let env = Envelope::new(ts(), msg);
    let json = encode(&env).unwrap();
    let decoded = decode(&json).unwrap();
    assert_eq!(env, decoded);
}

#[test]
fn round_trip_code_share() {
    let msg = Message::CodeShare(CodeSharePayload {
        snippet_id: "ejercicio-01".into(),
        title: "Ejemplo: Hola Mundo".into(),
        lang: "python".into(),
        content: "print('Hola, mundo')".into(),
        mode: CodeShareMode::Replace,
    });
    let env = Envelope::new(ts(), msg);
    let json = encode(&env).unwrap();
    let decoded = decode(&json).unwrap();
    assert_eq!(env, decoded);
}

#[test]
fn round_trip_code_share_append() {
    let msg = Message::CodeShare(CodeSharePayload {
        snippet_id: "ejercicio-02".into(),
        title: "Línea extra".into(),
        lang: "python".into(),
        content: "x = 42".into(),
        mode: CodeShareMode::Append,
    });
    let env = Envelope::new(ts(), msg);
    let json = encode(&env).unwrap();
    let decoded = decode(&json).unwrap();
    assert_eq!(env, decoded);
}

#[test]
fn round_trip_code_ack() {
    let msg = Message::CodeAck(CodeAckPayload {
        ref_id: sample_uuid(),
    });
    let env = Envelope::new(ts(), msg);
    let json = encode(&env).unwrap();
    let decoded = decode(&json).unwrap();
    assert_eq!(env, decoded);
}

#[test]
fn round_trip_code_clear() {
    let msg = Message::CodeClear(CodeClearPayload {});
    let env = Envelope::new(ts(), msg);
    let json = encode(&env).unwrap();
    let decoded = decode(&json).unwrap();
    assert_eq!(env, decoded);
}

#[test]
fn round_trip_msg_show_info() {
    let msg = Message::MsgShow(MsgShowPayload {
        text: "Tienen 10 minutos.".into(),
        level: MsgLevel::Info,
    });
    let env = Envelope::new(ts(), msg);
    let json = encode(&env).unwrap();
    let decoded = decode(&json).unwrap();
    assert_eq!(env, decoded);
}

#[test]
fn round_trip_msg_show_warning() {
    let msg = Message::MsgShow(MsgShowPayload {
        text: "¡Atención!".into(),
        level: MsgLevel::Warning,
    });
    let env = Envelope::new(ts(), msg);
    let json = encode(&env).unwrap();
    let decoded = decode(&json).unwrap();
    assert_eq!(env, decoded);
}

#[test]
fn round_trip_msg_ack() {
    let msg = Message::MsgAck(MsgAckPayload {
        ref_id: sample_uuid(),
    });
    let env = Envelope::new(ts(), msg);
    let json = encode(&env).unwrap();
    let decoded = decode(&json).unwrap();
    assert_eq!(env, decoded);
}

#[test]
fn round_trip_error() {
    let msg = Message::Error(ErrorPayload {
        ref_id: sample_uuid(),
        code: ErrorCode::InvalidPayload,
        message: "Campo obligatorio faltante".into(),
    });
    let env = Envelope::new(ts(), msg);
    let json = encode(&env).unwrap();
    let decoded = decode(&json).unwrap();
    assert_eq!(env, decoded);
}

// ---------------------------------------------------------------------------
// Validation tests
// ---------------------------------------------------------------------------

#[test]
fn decode_unsupported_version() {
    let json = r#"{"v":99,"id":"550e8400-e29b-41d4-a716-446655440000","ts":0,"type":"heartbeat","payload":{}}"#;
    let err = decode(json).unwrap_err();
    assert!(
        matches!(err, ProtocolError::UnsupportedVersion(99)),
        "expected UnsupportedVersion, got: {err}"
    );
}

#[test]
fn decode_invalid_json() {
    let err = decode("this is not json").unwrap_err();
    assert!(
        matches!(err, ProtocolError::Json(_)),
        "expected Json error, got: {err}"
    );
}

#[test]
fn decode_missing_fields() {
    let json = r#"{"v":1}"#;
    let err = decode(json).unwrap_err();
    assert!(
        matches!(err, ProtocolError::Json(_)),
        "expected Json error for missing fields, got: {err}"
    );
}

#[test]
fn encode_code_content_too_large() {
    let big_content = "x".repeat(MAX_CODE_CONTENT_SIZE + 1);
    let msg = Message::CodeShare(CodeSharePayload {
        snippet_id: "big".into(),
        title: "Big".into(),
        lang: "text".into(),
        content: big_content,
        mode: CodeShareMode::Replace,
    });
    let env = Envelope::new(ts(), msg);
    let err = encode(&env).unwrap_err();
    assert!(
        matches!(err, ProtocolError::CodeContentTooLarge { .. }),
        "expected CodeContentTooLarge, got: {err}"
    );
}

#[test]
fn decode_code_content_too_large() {
    let big_content = "x".repeat(MAX_CODE_CONTENT_SIZE + 1);
    let json = format!(
        r#"{{"v":1,"id":"550e8400-e29b-41d4-a716-446655440000","ts":0,"type":"code.share","payload":{{"snippet_id":"big","title":"Big","lang":"text","content":"{}","mode":"replace"}}}}"#,
        big_content
    );
    let err = decode(&json).unwrap_err();
    assert!(
        matches!(err, ProtocolError::CodeContentTooLarge { .. }),
        "expected CodeContentTooLarge, got: {err}"
    );
}

#[test]
fn encode_msg_text_too_large() {
    let big_text = "a".repeat(MAX_MSG_TEXT_SIZE + 1);
    let msg = Message::MsgShow(MsgShowPayload {
        text: big_text,
        level: MsgLevel::Info,
    });
    let env = Envelope::new(ts(), msg);
    let err = encode(&env).unwrap_err();
    assert!(
        matches!(err, ProtocolError::MsgTextTooLarge { .. }),
        "expected MsgTextTooLarge, got: {err}"
    );
}

#[test]
fn encode_message_total_too_large() {
    // A message whose serialized form exceeds 1 MiB.
    // We use a code.share with content just under the content limit
    // but pad extra fields to push the total over 1 MiB.
    let content = "x".repeat(MAX_CODE_CONTENT_SIZE);
    let big_title = "t".repeat(MAX_MESSAGE_SIZE); // push total over
    let msg = Message::CodeShare(CodeSharePayload {
        snippet_id: "overflow".into(),
        title: big_title,
        lang: "text".into(),
        content,
        mode: CodeShareMode::Replace,
    });
    let env = Envelope::new(ts(), msg);
    let err = encode(&env).unwrap_err();
    assert!(
        matches!(err, ProtocolError::MessageTooLarge { .. }),
        "expected MessageTooLarge, got: {err}"
    );
}

// ---------------------------------------------------------------------------
// Fixture file deserialization tests
// ---------------------------------------------------------------------------

macro_rules! fixture_test {
    ($name:ident, $file:expr, $variant:pat) => {
        #[test]
        fn $name() {
            let json = include_str!(concat!("fixtures/", $file));
            let envelope = decode(json).unwrap();
            assert_eq!(envelope.v, PROTOCOL_VERSION);
            assert!(
                matches!(envelope.message, $variant),
                "unexpected message variant: {:?}",
                envelope.message
            );
        }
    };
}

fixture_test!(fixture_hello, "hello.json", Message::Hello(_));
fixture_test!(
    fixture_auth_challenge,
    "auth_challenge.json",
    Message::AuthChallenge(_)
);
fixture_test!(fixture_auth_proof, "auth_proof.json", Message::AuthProof(_));
fixture_test!(fixture_auth_ok, "auth_ok.json", Message::AuthOk(_));
fixture_test!(fixture_heartbeat, "heartbeat.json", Message::Heartbeat(_));
fixture_test!(fixture_code_share, "code_share.json", Message::CodeShare(_));
fixture_test!(fixture_code_ack, "code_ack.json", Message::CodeAck(_));
fixture_test!(fixture_code_clear, "code_clear.json", Message::CodeClear(_));
fixture_test!(fixture_msg_show, "msg_show.json", Message::MsgShow(_));
fixture_test!(fixture_msg_ack, "msg_ack.json", Message::MsgAck(_));
fixture_test!(fixture_error, "error.json", Message::Error(_));

// ---------------------------------------------------------------------------
// Fixture content validation
// ---------------------------------------------------------------------------

#[test]
fn fixture_hello_content() {
    let json = include_str!("fixtures/hello.json");
    let envelope = decode(json).unwrap();
    if let Message::Hello(ref h) = envelope.message {
        assert_eq!(h.hostname, "PC-LAB-12");
        assert_eq!(h.os, "windows");
        assert_eq!(h.capabilities.len(), 2);
        assert!(h.capabilities.contains(&Capability::CodeView));
        assert!(h.capabilities.contains(&Capability::MsgShow));
    } else {
        panic!("expected Hello");
    }
}

#[test]
fn fixture_code_share_content() {
    let json = include_str!("fixtures/code_share.json");
    let envelope = decode(json).unwrap();
    if let Message::CodeShare(ref cs) = envelope.message {
        assert_eq!(cs.snippet_id, "ejercicio-01");
        assert_eq!(cs.lang, "python");
        assert_eq!(cs.mode, CodeShareMode::Replace);
    } else {
        panic!("expected CodeShare");
    }
}

// ---------------------------------------------------------------------------
// Error code serialization
// ---------------------------------------------------------------------------

#[test]
fn error_codes_round_trip() {
    let codes = vec![
        ErrorCode::UnsupportedVersion,
        ErrorCode::UnknownType,
        ErrorCode::InvalidPayload,
        ErrorCode::AuthRequired,
        ErrorCode::AuthFailed,
        ErrorCode::CapabilityMissing,
        ErrorCode::RateLimited,
        ErrorCode::MessageTooLarge,
    ];
    for code in codes {
        let json = serde_json::to_string(&code).unwrap();
        let back: ErrorCode = serde_json::from_str(&json).unwrap();
        assert_eq!(code, back);
    }
}

// ---------------------------------------------------------------------------
// Capability serialization
// ---------------------------------------------------------------------------

#[test]
fn capabilities_round_trip() {
    let caps = vec![
        Capability::CodeView,
        Capability::MsgShow,
        Capability::ScreenThumb,
        Capability::ScreenLock,
        Capability::ScreenUnlock,
        Capability::InputBlock,
        Capability::CodeCollect,
        Capability::FileCollect,
        Capability::AppList,
    ];
    let json = serde_json::to_string(&caps).unwrap();
    let back: Vec<Capability> = serde_json::from_str(&json).unwrap();
    assert_eq!(caps, back);
}
