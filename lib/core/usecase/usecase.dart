import 'package:fpdart/fpdart.dart';
import 'package:freezed_annotation/freezed_annotation.dart';
import '../error/failures.dart';

/// Base UseCase interface.
abstract interface class UseCase<P, R> {
  Future<Either<Failure, R>> call(P params);
}

/// NoParams for usecases that do not require parameters.
final class NoParams {
  const NoParams();
}
