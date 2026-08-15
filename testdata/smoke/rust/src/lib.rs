pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

pub fn mul(a: i32, b: i32) -> i32 {
    a * b
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_add() {
        assert_eq!(add(2, 3), 5);
    }

    #[test]
    fn test_mul() {
        assert_eq!(mul(2, 4), 8);
    }
}
