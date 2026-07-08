import 'package:freezed_annotation/freezed_annotation.dart';

part 'failures.freezed.dart';

@freezed
sealed class Failure with _$Failure {
  const factory Failure.server({
    required String message,
    int? statusCode,
  }) = ServerFailure;

  const factory Failure.cache({
    required String message,
  }) = CacheFailure;

  const factory Failure.network() = NetworkFailure;

  const factory Failure.notFound() = NotFoundFailure;

  const factory Failure.unknown({
    required String message,
  }) = UnknownFailure;
}
