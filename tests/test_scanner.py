from credlint.scanner import scan_text

AWS_KEY_ID = "AKIAIOSFODNN7EXAMPLE"


def test_aws_access_key_id_in_plain_text_is_reported():
    text = f'aws_access_key_id = "{AWS_KEY_ID}"\n'
    findings = scan_text(text, path="config/aws.ini")

    aws = [f for f in findings if f.rule_id == "aws-access-key-id"]
    assert len(aws) == 1
    finding = aws[0]
    assert finding.path == "config/aws.ini"
    assert finding.line == 1
    assert finding.preview.startswith(AWS_KEY_ID[:4])
    assert AWS_KEY_ID not in finding.preview
