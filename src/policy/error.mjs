export class PolicyError extends Error {
  constructor(message, details = {}) {
    super(message);
    this.name = 'PolicyError';
    this.details = details;
  }
}
