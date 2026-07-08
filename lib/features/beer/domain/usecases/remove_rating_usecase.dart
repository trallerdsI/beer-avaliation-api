import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../repositories/i_user_preferences_repository.dart';
import '../../../../core/usecase/usecase.dart';

class RemoveRatingParams {
  final int beerId;
  const RemoveRatingParams({required this.beerId});
}

class RemoveRatingUseCase implements UseCase<RemoveRatingParams, Unit> {
  RemoveRatingUseCase({required this.preferencesRepository});

  final IUserPreferencesRepository preferencesRepository;

  @override
  Future<Either<Failure, Unit>> call(RemoveRatingParams params) async {
    return preferencesRepository.removeUserRating(params.beerId);
  }
}
