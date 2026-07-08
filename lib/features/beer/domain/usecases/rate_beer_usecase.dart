import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../repositories/i_user_preferences_repository.dart';
import '../../../../core/usecase/usecase.dart';

class RateBeerParams {
  final int beerId;
  final int stars;
  const RateBeerParams({required this.beerId, required this.stars});
}

class RateBeerUseCase implements UseCase<RateBeerParams, Unit> {
  RateBeerUseCase({required this.preferencesRepository});

  final IUserPreferencesRepository preferencesRepository;

  @override
  Future<Either<Failure, Unit>> call(RateBeerParams params) async {
    final int clamped = params.stars.clamp(1, 5) as int;
    if (clamped != params.stars) {
      return Left(const Failure.unknown(message: 'Rating inválido'));
    }

    return preferencesRepository.setUserRating(params.beerId, clamped);
  }
}
